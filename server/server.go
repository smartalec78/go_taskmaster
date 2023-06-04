package main

import (
    "context"
    "io"
    "log"
    "ajb497/ptmp"
    "github.com/quic-go/quic-go"
    "time"
    "fmt"
    "strings"
)

var buff_incoming = make([]byte, 2048)
const HOST string = "localhost:10101" // TODO: move this to a config file
const LOGGING_ENABLED bool = true
const VALID_UNAME string = "Ed Ucational"
const VALID_PW string = "p@55w0rd" // because we believe in super high security here at Alec's Computer Code and Fishing Tackle Emporium

var active_proto_version uint16 = 1 // I've only made one of these so far
var exts_enabled = make([]uint16, 0) // And I have not yet needed to extend it beyond my original spec... mostly because I haven't even coded the entirety of the original spec yet.
var proto_versions_supported = make([]uint16, 1)
var timeout_permitted uint16 = 60 // You get one minute of idle-time in the connection.

// Some convenient member variables for the server that all functions can access
var rcvdMsg *ptmp.PTMP_Msg
var connectionEstablished bool = false
var qconn quic.Connection
var stream quic.Stream
var exit_program bool = false
var active_tasks []ptmp.T_Inf

// Initial setup of the QUIC connection.
func connect_to_client(addr string) (quic.Connection, error) {
    if LOGGING_ENABLED {
        log.Printf("Server is initializing")
    }
    // The time package was complaining about me trying to directly multiply a uint16 or int with time.Second, so I'm going with
    // this cludgey string-parsing method that I don't really like.
    timeout, _ := time.ParseDuration(fmt.Sprintf("%ds",timeout_permitted))
    my_config := quic.Config{MaxIdleTimeout: timeout}
    listener, err := quic.ListenAddr(addr, ptmp.GenerateTLSConfig(), &my_config)
    if err != nil {
        return nil, err
    }
    if LOGGING_ENABLED {
        log.Printf("Server finalizing setup.")
    }
    return listener.Accept(context.Background())
}

// This is the core function where the server will be spending most of its time.
func recv() error {
    // Await a connection from the client and proceed with the stream once that's done
    if LOGGING_ENABLED {
        log.Printf("Server is standing-by for a connection.")
    }
    var err_status error
    stream, err_status = qconn.AcceptStream(context.Background())
    
    if err_status != nil {
        return err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Connection established.")
    }
    // Continuously look for incoming messages.
    for false == exit_program {
        num_bytes_in, err_status := stream.Read(buff_incoming)
        if (err_status != nil) && (err_status!= io.ErrUnexpectedEOF) {
            log.Printf("Error attempting to read from client:\n\t%+v\n", err_status)
            return err_status
        }
        if LOGGING_ENABLED {
            log.Printf("Server has receved a message from the client.")
        }
        rcvdMsg = ptmp.DecodePacket(buff_incoming[0:num_bytes_in])
        // Once we've got the header decoded and the full message stored as rcvdMsg, we can go into our server-side logic
        // of what to actually do now that we've received something from the client.
        determine_response()
    }

    // Close-out the connection.
    connCtx := qconn.Context()
    <-connCtx.Done()

    return nil
}

// If something needs to get sent to the client, it can be provided here as a byte stream and it'll get shot right out.
func xmit(packet_out []byte) (int, error) {

	num_bytes_out, err_status := stream.Write(packet_out)
    if err_status != nil {
        log.Printf("Error writing to client: %+v\n", err_status)
        return 0, err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Wrote %d bytes to the client.\n", num_bytes_out)
    }
    return num_bytes_out, err_status
}

// This is the main server business logic function - this is where we go when we receive incoming
// messages and then decide how to proceed.
func determine_response() {
    // first part of the DFA is setting up the connection
    if connectionEstablished {
        // We can't decode the body until we know what the overall message type is
        // And our action is going to depend on what type of message we're receiving (and if it's in the right context)
        switch rcvdMsg.Hdr.Msg_Type_ID {
            case ptmp.REQUEST_CONNECTION:
                // To get into this switch/case, you need to be in an already-established connection, so sending
                // another Request_Connection at this point is contextually invalid.
                sendAck(ptmp.MSG_CONTEXT_INVALID)
            case ptmp.CREATE_NEW_TASK:
                incoming_contents := ptmp.DecodePayload[ptmp.Create_New_Task](rcvdMsg.Pld)
                if LOGGING_ENABLED {
                    log.Printf("\nReceived a new task from the client:\n\tTitle: %v\n\tList // Priority: %v // %v\n\tDescription: %v\n\n",
                            string(incoming_contents.Task_Title),
                            incoming_contents.Associated_List_ID,
                            incoming_contents.Priority_Value,
                            string(incoming_contents.Task_Description))
                }
                sendAck(ptmp.SINGULAR_MSG_SUCCESS)
            case ptmp.CLOSE_CONNECTION:
                incoming_contents := ptmp.DecodePayload[ptmp.Close_Connection](rcvdMsg.Pld)
                if LOGGING_ENABLED {
                    log.Printf("\nReceived a Close_Connection message.\n\tClient to await ack before closing: %v\n", incoming_contents.Will_Await_Ack)
                }
                if ptmp.Byte2Bool(incoming_contents.Will_Await_Ack) {
                    sendAck(ptmp.SINGULAR_MSG_SUCCESS)
                }
                exit_program = true // client said it's done, so we are too [since this is a demo program that only connects to our one special client]
            default:
                sendAck(ptmp.MSG_NOT_IMPLEMENTED)
        }
    } else {
        // the message we got better be a REQUEST_CONNECTION
        if rcvdMsg.Hdr.Msg_Type_ID == ptmp.REQUEST_CONNECTION {
            incoming_contents := ptmp.DecodePayload[ptmp.Request_Connection](rcvdMsg.Pld)
            the_uname := byteArray2Str(incoming_contents.Username[:])
            the_pw := byteArray2Str(incoming_contents.Password[:])
            if LOGGING_ENABLED {
                log.Printf("The username provided was '%v', password '%v'.", the_uname, the_pw)
            }
            // The trailing null bytes from the username and password byte arrays need to be trimmed out in order
            // to make the straight string comparison with stored values behave as expected
            uname_good := the_uname == VALID_UNAME
            pw_good := the_pw == VALID_PW
            sendConnRules(uname_good, pw_good)
            if uname_good && pw_good {
                connectionEstablished = true
            }
            if LOGGING_ENABLED {
                if connectionEstablished {
                    log.Printf("Connection has been established (username and password checked out).\n")
                } else {
                    log.Printf("Connection unable to be established.\n\tUsername received: %v\n\tUsername accepted: %v\n\tPassword Received: %v\n\tPassword accepted: %v\n\n",
                               the_uname,
                               VALID_UNAME,
                               the_pw,
                               VALID_PW)
                }
            }
        } else {
            // If the connection hasn't been established with the proper handshake, any message we received right now is out of context
            sendAck(ptmp.MSG_CONTEXT_INVALID)
        }
    }
}

func byteArray2Str(in_bytes []byte) string {
    return strings.Trim(string(in_bytes[:]), "\x00")
}

func addTaskToList(newTaskMsg ptmp.Create_New_Task) {
    title := byteArray2Str(newTaskMsg.Task_Title[:])
    description := byteArray2Str(newTaskMsg.Task_Description[:])
    thisTask := ptmp.T_Inf{
                      Task_Reference_Number: uint16(len(active_tasks)),
                      Task_Priority_Value: newTaskMsg.Priority_Value,
                      Length_of_Title: byte(len(title)),
                      Task_Title: []byte(title),
                      Description_Length: uint16(len(description)),
                      Task_Description: []byte(description),
                      Completion_Status: ptmp.Bool2Byte(false),
    }
    active_tasks = append(active_tasks, thisTask)
}

func sendAck(response_code uint16) {
    ack := ptmp.Prep_Acknowledgment(response_code, rcvdMsg.Hdr.Msg_Type_ID)
    encoded := ptmp.EncodePacket(ack)
    xmit(encoded)
}

func sendConnRules(uname_ok bool, pw_ok bool) {
    conn_rules := ptmp.Prep_Connection_Rules(uname_ok, pw_ok, active_proto_version, []uint16{})
    encoded := ptmp.EncodePacket(conn_rules)
	xmit(encoded)
}

func main() {
	proto_versions_supported[0] = active_proto_version
	var err error
    qconn, err = connect_to_client(HOST)
    if LOGGING_ENABLED {
        log.Printf("Server just initialized, error is %+v", err)
    }
    recv()

    //time.Sleep(time.Second * 2)
}
