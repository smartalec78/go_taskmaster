package main

import (
    "io"
    "log"
    "ajb497/ptmp"
    "time"
    "strings"
    "net"
)

var buff_incoming = make([]byte, 2048) // Incoming buffer is intentionally oversized to ensure no possible issues with running out of space when receiving a message
const HOST string = "localhost:10101" // Per assignment specification, server hard-codes the port number.
const BASE_PROTO string = "tcp" // I had been implementing this using QUIC originally, but I figured that might count as a 3rd party library, so went down to just TCP
const LOGGING_ENABLED bool = true
const VALID_UNAME string = "Ed Ucational"
const VALID_PW string = "p@55w0rd" // because we believe in super high security here at Alec's Computer Code and Fishing Tackle Emporium

var active_proto_version uint16 = 1 // I've only made one of these so far
var exts_enabled = make([]uint16, 0) // And I have not yet needed to extend it beyond my original spec... mostly because I haven't even coded the entirety of the original spec yet.
var proto_versions_supported = make([]uint16, 1)
var timeout_permitted uint16 = 60 // Not actually used at the moment, but timeout as a concept would exist in fuller implementations of the spec

// Some convenient member variables for the server that all functions can access
var rcvdMsg *ptmp.PTMP_Msg
var connectionEstablished bool = false
var conn net.Conn
var exit_program bool = false
var active_tasks []ptmp.T_Inf

// Initial setup of the connection.
func connect_to_client(addr string) (net.Conn, error) {
    if LOGGING_ENABLED {
        log.Printf("Server is initializing")
    }

    listener, err_status := net.Listen(BASE_PROTO, addr)

    if err_status != nil {
        return nil, err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Server finalizing setup.")
    }
    this_conn, err_status := listener.Accept()
    if err_status != nil {
        return nil, err_status
    }

    return this_conn, nil
}

// This is the core function where the server will be spending most of its time.
func recv() error {

    // Continuously look for incoming messages.
    for false == exit_program {
        // any incoming data gets put into our receipt buffer
        num_bytes_in, err_status := conn.Read(buff_incoming)
        if (err_status != nil) && (err_status!= io.ErrUnexpectedEOF) {
            log.Printf("Error attempting to read from client:\n\t%+v\n", err_status)
            return err_status
        }
        if LOGGING_ENABLED {
            log.Printf("Server has receved a message from the client.")
        }
        // By decoding the packet, we get the header information, and once we have that, we can go into our normal server logic of what to do about each message type (DFA).
        rcvdMsg = ptmp.DecodePacket(buff_incoming[0:num_bytes_in])

        // Once we've got the header decoded and the full message stored as rcvdMsg, we can go into our server-side logic
        // of what to actually do now that we've received something from the client.
        determine_response()
    }
    conn.Close()
    return nil
}

// If something needs to get sent to the client, it can be provided here as a byte stream and it'll get shot right out.
func xmit(packet_out []byte) (int, error) {

	num_bytes_out, err_status := conn.Write(packet_out)
    if err_status != nil {
        log.Printf("Error writing to client: %+v\n", err_status)
        return 0, err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Wrote a response to the client.\n")
    }
    return num_bytes_out, err_status
}

// This is the main server business logic function - this is where we go when we receive incoming
// messages and then decide how to proceed (DFA).
func determine_response() {
    // first part of the DFA is setting up the connection, so if that's not done yet, the protocol is in a different state.
    if connectionEstablished {
        // We can't decode the body until we know what the overall message type is
        // And our action is going to depend on what type of message we're receiving (and if it's in the right context)
        switch rcvdMsg.Hdr.Msg_Type_ID {
            case ptmp.REQUEST_CONNECTION:
                // To get into this switch/case, you need to be in an already-established connection, so sending
                // another Request_Connection at this point is contextually invalid.
                sendAck(ptmp.MSG_CONTEXT_INVALID)
            case ptmp.CREATE_NEW_TASK:
                // we'll take in the new task and add it into our active task list so that it can be
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
                connectionEstablished = false // this shouldn't be needed, but just to be safe, we're declaring the connection officially dis-established
            case ptmp.QUERY_TASKS:
                sendTaskInfo() // We're in one of the few messages that doesn't get responded-to with an ack, so there's special logic to respond to this one
            case ptmp.REMOVE_TASK:
                incoming_contents := ptmp.DecodePayload[ptmp.Remove_Tasks](rcvdMsg.Pld)
                removeTasks(incoming_contents.Tasks_To_Remove, ptmp.Byte2Bool(incoming_contents.Permit_Remove_Incomplete)) // handles its own ack-sending
            case ptmp.MARK_TASK_COMPLETED:
                incoming_contents := ptmp.DecodePayload[ptmp.Mark_Task_Completed](rcvdMsg.Pld)
                completeTask(incoming_contents.List_ID, incoming_contents.Task_To_Mark) // handles its own ack-sending
            default:
                if LOGGING_ENABLED {
                    log.Printf("Received a message of type %v that we don't have implemented.\n", rcvdMsg.Hdr.Msg_Type_ID)
                }
                // If you made it here, you sent a message with an ID in the header that I do not yet have a server implementation to handle.
                sendAck(ptmp.MSG_NOT_IMPLEMENTED)
        }
    } else {
        // the message we got better be a REQUEST_CONNECTION
        if rcvdMsg.Hdr.Msg_Type_ID == ptmp.REQUEST_CONNECTION {
            incoming_contents := ptmp.DecodePayload[ptmp.Request_Connection](rcvdMsg.Pld)
            // The trailing null bytes from the username and password byte arrays need to be trimmed out in order
            // to make the straight string comparison with stored values behave as expected
            the_uname := byteArray2Str(incoming_contents.Username[:])
            the_pw := byteArray2Str(incoming_contents.Password[:])
            if LOGGING_ENABLED {
                log.Printf("The username provided was '%v', password '%v'.", the_uname, the_pw)
            }
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
            // If the connection hasn't been established with the proper handshake, any message we received right now that isn't a request connection is out of context
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
    // For convenience, we'll store tasks in the same format that the Task_Information message will look for when sending info back to the client.
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
    active_tasks = append(active_tasks, thisTask) // record this task as actually being on our list of tasks
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
    if len(active_tasks) > 0 {
        // The original plan for these was that each message would have as many task information
        // structs packed into them as possible, but I'm reevaluating that and preferring
        // to just send one task information structure per message
        for ii := len(active_tasks)-1; ii >= 0; ii-- {
            tinfo := ptmp.Prep_Task_Information(active_tasks[ii:ii+1],byte(ii))
            encoded := ptmp.EncodePacket(tinfo)
            xmit(encoded)
            time.Sleep(250*time.Millisecond) // Testing has made me concerned about each side sending messages too quickly and the receiver could end up missing something, so I've put this in to slow down on repeated sends
    }
    } else {
        // no tasks to send, but client still expects to see a response message, so send an ACK with a relevant code
        sendAck(ptmp.UNABLE_TO_COMPLY)
    }

}

// Go through our task list and remove the tasks with the specified IDs.
func removeTasks(task_ids []uint16, permit_incomplete bool) {
    // Loop through our tasks list (backwards, since we're relying on its length and chopping items out of it).
    for ii := len(active_tasks)-1; ii >= 0; ii-- {
        // See if we can find the matching task ID in our list of tasks to remove.
        for jj := len(task_ids)-1; jj >= 0; jj-- {
            if active_tasks[ii].Task_Reference_Number == task_ids[jj] &&
               (ptmp.Byte2Bool(active_tasks[ii].Completion_Status) || permit_incomplete){
                   // in here, the current task ID matches one of the IDs specified for removal, and it is considered valid to remove it

                   // I looked at a few different ways to remove items from slices in go, and this "append everything except the item to be removed"
                   // was the one that was clearest to me how it was being done, so that's what I felt safest implementing
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
                break
            }
        }
    }
    // If any tasks are left in the list of what was supposed to be removed, that means we didn't find it (or it was invalid to remove it, which I'm classifying as the same error state as it simply not existing).
    if len(task_ids) > 0 {
        sendAck(ptmp.TASK_DOES_NOT_EXIST)
    } else {
        sendAck(ptmp.SINGULAR_MSG_SUCCESS)
    }
}

func completeTask(listId uint16, taskId uint16) {
    // Again, I omitted the list-management messages, so only list ID 1 is valid for this demonstration of the protocol.
    if listId != 1 {
        sendAck(ptmp.LIST_DOES_NOT_EXIST)
        return
    }

    foundRightTask := false
    // loop through the list, see if we find the id we're looking for
    for ii := 0; ii < len(active_tasks); ii++ {
        if active_tasks[ii].Task_Reference_Number == taskId {
            foundRightTask = true
            active_tasks[ii].Completion_Status = ptmp.Bool2Byte(true) // set its completion status to true in our current tracking list
            break
        }
    }

    // Send the appropriate ACK code depending on whether the task was successfully found in our list.
    if !foundRightTask {
        sendAck(ptmp.TASK_DOES_NOT_EXIST)
    } else {
        sendAck(ptmp.SINGULAR_MSG_SUCCESS)
    }

}

func main() {
	proto_versions_supported[0] = active_proto_version
	var err error
	// set up the server to listen for incoming connections, and then receive (and handle) incoming messages until the client says we're done
    conn, err = connect_to_client(HOST)
    if LOGGING_ENABLED {
        log.Printf("Server just initialized, error is %+v", err)
    }
    recv()
}
