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
                // we'll take in th new task and add it into our active task list so that it can be
                // referenced in other traffic with the client.
                incoming_contents := ptmp.DecodePayload[ptmp.Create_New_Task](rcvdMsg.Pld)
                sendAck(addTaskToList(*incoming_contents))
            case ptmp.CLOSE_CONNECTION:
                incoming_contents := ptmp.DecodePayload[ptmp.Close_Connection](rcvdMsg.Pld)
                if LOGGING_ENABLED {
                    log.Printf("\nReceived a Close_Connection message.\n\tClient to await ack before closing: %v\n", incoming_contents.Will_Await_Ack)
                }
                // We'll only bother sending the ACK if the client said they cared about waiting for it.
                if ptmp.Byte2Bool(incoming_contents.Will_Await_Ack) {
                    sendAck(ptmp.SINGULAR_MSG_SUCCESS)
                }
                exit_program = true // client said it's done, so we are too [since this is a demo program that only connects to our one special client]
                connectionEstablished = false // this shouldn't be needed, but just to be safe, we're declaring the connection officially closed
            case ptmp.QUERY_TASKS:
                sendTaskInfo()
            case ptmp.REMOVE_TASK:
                incoming_contents := ptmp.DecodePayload[ptmp.Remove_Tasks](rcvdMsg.Pld)
                removeTasks(incoming_contents.Tasks_To_Remove, ptmp.Byte2Bool(incoming_contents.Permit_Remove_Incomplete))
            case ptmp.MARK_TASK_COMPLETED:
                incoming_contents := ptmp.DecodePayload[ptmp.Mark_Task_Completed](rcvdMsg.Pld)
                completeTask(incoming_contents.List_ID, incoming_contents.Task_To_Mark)
            default:
                if LOGGING_ENABLED {
                    log.Printf("Received a message of type %v that we don't have implemented.\n", rcvdMsg.Hdr.Msg_Type_ID)
                }
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
            // We still send a connection rules message in response even if the username and password are not valid, but we do note that fact
            // in the response message.
            sendConnRules(uname_good, pw_good)

            // The connection is only considered established once the username and password combo checks-out, otherwise, the client will
            // need to send another connection request and retry the username/password combo.
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
    // Used for handling incoming message contents as strings, and removes those pesky trailing null bytes that may or may not be present depending on the field.
    return strings.Trim(string(in_bytes[:]), "\x00")
}

func addTaskToList(newTaskMsg ptmp.Create_New_Task) uint16 {
    title := byteArray2Str(newTaskMsg.Task_Title[:])
    // It shouldn't be possible for the title
    // to be beyond the maximum length at this point, but
    // it was the only thing I could think of to demonstrate
    // the use of the response code
    if uint16(len(title)) > ptmp.TITLE_MAX_LENGTH {
        return ptmp.INVALID_NAME
    }

    // and another error code you could get is trying to add something to a list
    // that doesn't exist
    // Since I'm not planning to implement the bulk of the list management stuff,
    // we'll just say that list number 1 is the only valid one to add tasks to,
    // and any other list number specified will result in an error
    if 1 != newTaskMsg.Associated_List_ID {
        return ptmp.LIST_DOES_NOT_EXIST
    }
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
    if LOGGING_ENABLED {
        log.Printf("\nNew task received from client and being added to the list:\n\tTitle: %v\n\tList // Priority: %v // %v\n\tDescription: %v\n\n",
                   title,
                   newTaskMsg.Associated_List_ID,
                   newTaskMsg.Priority_Value,
                   description)
    }
    active_tasks = append(active_tasks, thisTask)
    return ptmp.SINGULAR_MSG_SUCCESS
}

// Shoot off an ACK message back to the client with the specified response code.
func sendAck(response_code uint16) {
    ack := ptmp.Prep_Acknowledgment(response_code, rcvdMsg.Hdr.Msg_Type_ID)
    encoded := ptmp.EncodePacket(ack)
    xmit(encoded)
}

// To be sent in response to a Connection_Request message, gives some requirements for how the session will go.
func sendConnRules(uname_ok bool, pw_ok bool) {
    conn_rules := ptmp.Prep_Connection_Rules(uname_ok, pw_ok, active_proto_version, []uint16{})
    encoded := ptmp.EncodePacket(conn_rules)
	xmit(encoded)
}

func sendTaskInfo() {
    // The original plan for these was that each message would have as many task information
    // structs packed into them as possible, but I'm reevaluating that and preferring
    // to just send one task information structure per message
    for ii := len(active_tasks)-1; ii >= 0; ii-- {
        tinfo := ptmp.Prep_Task_Information(active_tasks[ii:ii+1],byte(ii))
        encoded := ptmp.EncodePacket(tinfo)
        xmit(encoded)
        time.Sleep(250*time.Millisecond) // I'm concerned about each side sending messages too quickly and the receiver could end up missing something, so I've put this in to slow down on repeated sends
    }
}

func removeTasks(task_ids []uint16, permit_incomplete bool) {
    for ii := len(active_tasks)-1; ii >= 0; ii-- {
        for jj := len(task_ids)-1; jj >= 0; jj-- {
            if active_tasks[ii].Task_Reference_Number == task_ids[jj] &&
               (ptmp.Byte2Bool(active_tasks[ii].Completion_Status) || permit_incomplete){
                   temp_arr := []uint16{}
                   for kk := 0; kk < len(task_ids); kk++ {
                       if kk != jj {
                           temp_arr = append(temp_arr, task_ids[kk])
                       }
                   }
                   task_ids = temp_arr

                   temp_arr2 := []ptmp.T_Inf{}
                   for kk := 0; kk < len(active_tasks); kk++ {
                       if kk != ii {
                           temp_arr2 = append(temp_arr2, active_tasks[kk])
                       }
                   }
                   active_tasks = temp_arr2
                   /*
                _, task_ids = task_ids[jj], task_ids[:jj]
                _, active_tasks = active_tasks[ii], active_tasks[:ii]*/
                break
            }
        }
    }
    if len(task_ids) > 0 {
        sendAck(ptmp.TASK_DOES_NOT_EXIST)
    } else {
        sendAck(ptmp.SINGULAR_MSG_SUCCESS)
    }
}

func completeTask(listId uint16, taskId uint16) {
    if listId != 1 {
        sendAck(ptmp.LIST_DOES_NOT_EXIST)
        return
    }
    foundRightTask := false
    // loop through the list, see if we find the id we're looking for
    for ii := 0; ii < len(active_tasks); ii++ {
        if active_tasks[ii].Task_Reference_Number == taskId {
            foundRightTask = true
            active_tasks[ii].Completion_Status = ptmp.Bool2Byte(true)
            break
        }
    }
    if !foundRightTask {
        sendAck(ptmp.TASK_DOES_NOT_EXIST)
    } else {
        sendAck(ptmp.SINGULAR_MSG_SUCCESS)
    }

}

func main() {
	proto_versions_supported[0] = active_proto_version
	var err error
    qconn, err = connect_to_client(HOST)
    if LOGGING_ENABLED {
        log.Printf("Server just initialized, error is %+v", err)
    }
    recv()
}
