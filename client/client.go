package main

import (
    "context"
    "crypto/tls"
    "io"
    "log"
    "ajb497/ptmp"
    "github.com/quic-go/quic-go"
    "time"
)

var buff_incoming = make([]byte, 2048)
const HOST string = "localhost:10101"
var PRINT_MSGS bool = true
var connection_established bool = false


func connect_to_server(host string) (quic.Stream, error) {
    tls_conf := &tls.Config{
        InsecureSkipVerify: true,
        NextProtos:         []string{"quic-security-setup"},
    }
    conn2srv, err_status := quic.DialAddr(host, tls_conf, nil)
    if err_status != nil {
        return nil, err_status
    }

    return conn2srv.OpenStreamSync(context.Background())
}

func recv(stream quic.Stream) (int, error) {

    num_bytes_in, err_status := stream.Read(buff_incoming)
    if (err_status != nil) && (err_status != io.ErrUnexpectedEOF) {
        log.Printf("ERROR GETTING SERVER RESPONSE %+v", err_status)
        return 0, err_status
    }

    pckt := ptmp.DecodePacket(buff_incoming[0:num_bytes_in])
//     log.Printf("RECEIVED\n----------\n%+v\n----------", pckt)
    num_subsequent := determine_action(pckt)
    return num_subsequent, nil
}

func determine_action(packet_in* ptmp.PTMP_Msg) int {
    switch packet_in.Hdr.Msg_Type_ID {
        case ptmp.CONNECTION_RULES:
            if connection_established {
                log.Printf("This is weird - we shouldn't be getting a Connection_Rules message since we already got the connection established...\n")
            }
            connection_established = true
            received_contents := ptmp.DecodePayload[ptmp.Connection_Rules](packet_in.Pld)
            if PRINT_MSGS {
                log.Printf("Received a Connection_Rules message, contents follow:\n\t%+v\n", received_contents)
            }
        case ptmp.ACKNOWLEDGMENT:
            received_contents := ptmp.DecodePayload[ptmp.Acknowledgment](packet_in.Pld)
            if PRINT_MSGS {
                log.Printf("Received an acknowledgement with code %v responding to our %v message.", received_contents.Response_Code, received_contents.ID_Responding_To)
            }
        case ptmp.TASK_INFORMATION:
            received_contents := ptmp.DecodePayload[ptmp.Task_Information](packet_in.Pld)
            if PRINT_MSGS {
                log.Printf("Received a Task_Information message:\n")
                for ii := 0; ii < len(received_contents.Task_Infos); ii++ {
                    printTinfo(received_contents.Task_Infos[ii])
                }
                log.Printf("Expecting %v more Task_Information messages to follow.\n", packet_in.Hdr.Msgs_To_Follow)
            }

        default:
            if PRINT_MSGS {
                log.Printf("\n\tReceived a message of type %v for which we don't have any actions defined.\n\n", packet_in.Hdr.Msg_Type_ID)
            }
    }
    return int(packet_in.Hdr.Msgs_To_Follow)
}

func xmit(stream quic.Stream, ptmp_out ptmp.PTMP_Msg, expect_response bool) (int, error) {
    serialized := ptmp.EncodePacket(ptmp_out)
    num_bytes_out, err_status := stream.Write(serialized)
    if err_status != nil {
        log.Printf("Error writing to server: %+v", err_status)
        return 0, err_status
    }
    log.Printf("Message type %v just sent to the server.\n", ptmp_out.Hdr.Msg_Type_ID)
    if expect_response{
        num_to_follow := 1
        for num_to_follow > 0 {
            num_to_follow, err_status = recv(stream)
        }
//         num_to_follow := recv(stream) // following a transmission, the server is supposed to respond (in most cases, at least)
    }
    time.Sleep(250 * time.Millisecond) // after each transmission, give the server some time before we flood it with additional traffic
    return num_bytes_out, err_status
}

func printTinfo(tinfo ptmp.T_Inf) {
    log.Printf("\n\tReference Number: %v\n\tPriority: %v\n\tTitle: %v\n\tDescription: %v\n\tCompletion: %v\n",
               tinfo.Task_Reference_Number,
               tinfo.Task_Priority_Value,
               string(tinfo.Task_Title[:]),
               string(tinfo.Task_Description),
               tinfo.Completion_Status > 0)
}

func main() {
    connection, _ := connect_to_server(HOST)
    req_conn := ptmp.Prep_Request_Connection("Ed Ucational", "p@55w0rd", 42, []uint16{1}, []uint16{})
    new_task := ptmp.Prep_Create_New_Task(1, 1000, "Grade this assignment", "You should give Alec an A for doing such an awesome job with this project!")
    xmit(connection, req_conn, true)
//     recv(connection)
	xmit(connection, new_task, true)

    new_task = ptmp.Prep_Create_New_Task(2, 1000, "Reject this!", "This is specifying a list that doesn't exist, so it should get rejected.")
    xmit(connection, new_task, true)

    new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be another task", "This is the second successful task, I hope.")
    xmit(connection, new_task, true)


    new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be yet another task", "This is the third successful task, I hope.")
    xmit(connection, new_task, true)

//     recv(connection)
    querier := ptmp.Prep_Query_Tasks(0, 50000)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Mark_Task_Completed(1, 1), true)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Remove_Tasks(true, uint16(1), []uint16{2}), true)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Close_Connection(false), false)


    connection.Close()
    <-connection.Context().Done()
}
