package main

import (
    "io"
    "log"
    "ajb497/ptmp"
    "time"
    "net"
    "os"
    "bufio"
    "strings"
)

var buff_incoming = make([]byte, 2048)
const CONFIG_FILENAME string = "client.cfg"
var host string
var PRINT_MSGS bool = true
var connection_established bool = false
const BASE_PROTO string = "tcp" // I had been planning on implementing this with QUIC, but the instruction about not using any libraries more advanced than the language's socket APIs made me switch to just plain unsecure TCP

func readConfig() {
    fileHandle, err_status := os.Open(CONFIG_FILENAME)
    if err_status != nil {
        log.Printf("Error reading %v:\n%v\n",CONFIG_FILENAME, err_status)
        return
    }
    rdr := bufio.NewReader(fileHandle)
    strsIn := []string{}
    hit_end := false
    for false == hit_end {
        thisLine, this_err := rdr.ReadString('\n')
        if this_err != nil {
            if this_err == io.EOF {
                hit_end = true
            } else {
                log.Printf("Error reading line from file:\n%v\n", this_err)
                return
            }
        }
        strsIn = append(strsIn, thisLine)
    }
    fileHandle.Close()
    // for now, I only plan on having one item in the config
    host = strings.Trim(strsIn[0], "\n")
}

func connect_to_server() (net.Conn, error) {

    //
    conn, err_status := net.Dial(BASE_PROTO, host)
    if err_status != nil {
        log.Printf("connection error (attempted host %v)", host)
        return nil, err_status
    }

    return conn, nil //conn2srv.OpenStreamSync(context.Background())
}

func recv(conn net.Conn) (int, error) {

    num_bytes_in, err_status := conn.Read(buff_incoming)
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

func xmit(conn net.Conn, ptmp_out ptmp.PTMP_Msg, expect_response bool) (int, error) {
    serialized := ptmp.EncodePacket(ptmp_out)
    num_bytes_out, err_status := conn.Write(serialized)
    if err_status != nil {
        log.Printf("Error writing to server: %+v", err_status)
        return 0, err_status
    }
    log.Printf("Message type %v just sent to the server.\n", ptmp_out.Hdr.Msg_Type_ID)
    if expect_response{
        num_to_follow := 1
        for num_to_follow > 0 {
            num_to_follow, err_status = recv(conn)
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
    readConfig()
    connection, _ := connect_to_server()//HOST)

    // As I haven't dealt with user-input scanning in Go yet, please enjoy this
    // canned conversation between the client and the server:

    req_conn := ptmp.Prep_Request_Connection("Ed Ucational", "p@55w0rd", 42, []uint16{1}, []uint16{})

    xmit(connection, req_conn, true)

    new_task := ptmp.Prep_Create_New_Task(1, 1000, "Grade this assignment", "You should give Alec an A for doing such an awesome job with this project!")
	xmit(connection, new_task, true)

    new_task = ptmp.Prep_Create_New_Task(2, 1000, "Reject this!", "This is specifying a list that doesn't exist, so it should get rejected.")
    xmit(connection, new_task, true)

    new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be another task", "This is the second successful task, I hope.")
    xmit(connection, new_task, true)


    new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be yet another task", "This is the third successful task, I hope.")
    xmit(connection, new_task, true)

    querier := ptmp.Prep_Query_Tasks(0, 50000)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Mark_Task_Completed(1, 1), true)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Remove_Tasks(true, uint16(1), []uint16{2}), true)
    xmit(connection, querier, true)
    xmit(connection, ptmp.Prep_Close_Connection(false), false)


    connection.Close()

}
