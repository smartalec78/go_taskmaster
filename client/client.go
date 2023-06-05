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
    "fmt"
    "strconv"
)

var buff_incoming = make([]byte, 2048)
const CONFIG_FILENAME string = "client.cfg"
var host string
var PRINT_MSGS bool = true
var connection_established bool = false
var connection net.Conn
var demo_mode bool = false
const BASE_PROTO string = "tcp" // I had been implementing this with QUIC, but the instruction about not using any libraries more advanced than the language's socket APIs made me switch to just plain unsecure TCP
var input_scanner *bufio.Scanner

func readConfig() {
    // The only item read from the configuration file for this demo is the host name/port number
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
    if len(strsIn) > 1 && strings.Trim(strsIn[1], "\n") == "DEMO" {
        demo_mode = true
    }
}

func connect_to_server() (net.Conn, error) {

    // Very straightforward, specify our protocol (TCP) and put in the host ID and go ahead and connect.
    conn, err_status := net.Dial(BASE_PROTO, host)
    if err_status != nil {
        log.Printf("connection error (attempted host %v):\n%v\n", host, err_status)
        return nil, err_status
    }

    return conn, nil
}

func recv(conn net.Conn) (int, error) {

    // This receive function is called whenever the client expects to be getting a message from the server...
    // Given more time, I would set up the listener as a second thread, but since this is my first time programming with go,
    // it would take me a while to get that set up correctly and robustly
    num_bytes_in, err_status := conn.Read(buff_incoming)
    if (err_status != nil) && (err_status != io.ErrUnexpectedEOF) {
        log.Printf("ERROR GETTING SERVER RESPONSE %+v", err_status)
        return 0, err_status
    }

    pckt := ptmp.DecodePacket(buff_incoming[0:num_bytes_in]) // Utilize the common library's decoder function to get at least the heaeder decoded.
    num_subsequent := determine_action(pckt) // Do we expect to get more messages in sequence in this transaction?
    return num_subsequent, nil
}

// Given a PTMP_Msg, take some action and return an integer indicating how many more messages are expected in this sequence.
func determine_action(packet_in* ptmp.PTMP_Msg) int {
    switch packet_in.Hdr.Msg_Type_ID {
        case ptmp.CONNECTION_RULES:
            if connection_established {
                log.Printf("This is weird - we shouldn't be getting a Connection_Rules message since we already got the connection established...\n")
                // The protocol doesn't define any action that the client should take in response to an out-of-context message from the server, so just mark it as weird and move on.
            }
            received_contents := ptmp.DecodePayload[ptmp.Connection_Rules](packet_in.Pld)
            connection_established = ptmp.Byte2Bool(received_contents.Username_Ok) && ptmp.Byte2Bool(received_contents.Password_Ok)
            if PRINT_MSGS {
                log.Printf("Received a Connection_Rules message, contents follow:\n\t%+v\n", received_contents)
            }
        case ptmp.ACKNOWLEDGMENT:
            // Generally the most common type of message we can expect from the server
            received_contents := ptmp.DecodePayload[ptmp.Acknowledgment](packet_in.Pld)
            if PRINT_MSGS {
                log.Printf("Received an acknowledgement with code %v responding to our %v message.", received_contents.Response_Code, received_contents.ID_Responding_To)
            }
        case ptmp.TASK_INFORMATION:
            // If we queried the tasks held on the server and it responded positively, this will tell us what tasks are held on the server.
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

func xmit(conn net.Conn, ptmp_out ptmp.PTMP_Msg) (int, error) {
    // Input params are the connection over which the message should be sent and the PTMP_Msg to be sent

    // There's only one message type that we might not expect a response from the server for
    var expect_response bool
    if ptmp_out.Hdr.Msg_Type_ID == ptmp.CLOSE_CONNECTION {
        sending_contents := ptmp.DecodePayload[ptmp.Close_Connection](ptmp_out.Pld)
        expect_response = ptmp.Byte2Bool(sending_contents.Will_Await_Ack)
    } else {
        expect_response = true
    }

    serialized := ptmp.EncodePacket(ptmp_out) // get the byte-array representation of the full message
    num_bytes_out, err_status := conn.Write(serialized) // send the message
    if err_status != nil {
        log.Printf("Error writing to server: %+v", err_status)
        return 0, err_status
    }
    if PRINT_MSGS {
        log.Printf("Message type %v just sent to the server.\n", ptmp_out.Hdr.Msg_Type_ID)
    }
    if expect_response{
        // Unlike on the server-side where listening for incoming messages is its primary role, the client's main job is
        // sending messages to the server, so going into receive mode is only done in response to us sending a message to the server.

        num_to_follow := 1 // if we've decided that a response is expected, then we expect at least one message
                           // but when we receive that message, it may say that there are more messages to come in the same sequence,
                           // and if that's the case, then we should say in receive mode until there are no more messages expected to come in
        for num_to_follow > 0 {
            num_to_follow, err_status = recv(conn)
        }
    }
    time.Sleep(250 * time.Millisecond) // after each transmission, give the server some time before we flood it with additional traffic
    return num_bytes_out, err_status
}

func printTinfo(tinfo ptmp.T_Inf) {
    // helper function to print out the details of the tasks that we've received info on from the server
    log.Printf("\n\tReference Number: %v\n\tPriority: %v\n\tTitle: %v\n\tDescription: %v\n\tCompletion: %v\n",
               tinfo.Task_Reference_Number,
               tinfo.Task_Priority_Value,
               string(tinfo.Task_Title[:]),
               string(tinfo.Task_Description),
               tinfo.Completion_Status > 0)
}

func prompt_for_str(prompt_in string, max_length int) string {
    // Show the user a prompt and then make sure their response matches our specified length requirements,
    outVal := ""
    for len(outVal) < 1 || len(outVal) > max_length {
        fmt.Printf(prompt_in)
        input_scanner.Scan()
        outVal = input_scanner.Text()
        fmt.Printf("\n")
        if len(outVal) < 1 || len(outVal) > max_length {
            fmt.Printf("\nYour response must be between 1 and %v characters long.", 1, max_length)
        } else {
            break
        }
    }
    return outVal
}

func prompt_for_int(prompt_in string, min_val, max_val int) int {
    // show the user a prompt and make sure their response matches our min/max requirements
    curr_out := min_val - 1
    for curr_out < min_val || curr_out > max_val {
        fmt.Printf(prompt_in)
        input_scanner.Scan()
        temp_out, err_status := strconv.ParseInt(input_scanner.Text(), 10, 0)
        if err_status != nil {
            fmt.Printf("There was a problem parsing your input (%v), please try again.\n",err_status)
            curr_out = min_val - 1
        } else {
            curr_out = int(temp_out)
        }
        if curr_out < min_val || curr_out > max_val {
            fmt.Printf("Please choose an option from %v to %v.\n", min_val, max_val)
        }
    }
    fmt.Printf("\n")
    return curr_out
}

func read_input() {
    // This user-input portion is by far the least-tested component of the project.
    input_scanner = bufio.NewScanner(os.Stdin) // set up a way to read user input
    for false == connection_established {
        // until we've established the connection, we need to keep on asking for login credentials
        uname := prompt_for_str("Please tell me the username you'd like to use: ", int(ptmp.USERNAME_SIZE))
        pw := prompt_for_str("And the password: ", int(ptmp.PASSWORD_SIZE))
        xmit(connection, ptmp.Prep_Request_Connection(uname, pw, 0, []uint16{1}, []uint16{})) // determining whether or not the connection has been established is part of
                                                                                              // the client-side logic handling responses from the server
    }
    quit_program := false
    for false == quit_program {
        curr_choice := prompt_for_int("\nWould you like to\n\t1. Make a new task\n\t2. See current tasks\n\t3. Mark a task completed\n\t4. Remove a task\n\t5. Quit\n", 1, 5)

        switch curr_choice {
            case 1:
                // make a new task
                // need:
                // list ID
                // priority value
                // title
                // description
                list_id := prompt_for_int("\nPlease enter the list number you'd like to use (only '1' works, but others can be used to generate an error response): ", 0, 255)
                priority_val := prompt_for_int("\nAnd what is the priority value of this task: ", 1, 60000)
                title := prompt_for_str("\nWhat is the task's title: ", int(ptmp.TITLE_MAX_LENGTH))
                description := prompt_for_str("\nTask description: ", int(ptmp.DESCRIPTION_MAX_LENGTH))
                xmit(connection, ptmp.Prep_Create_New_Task(uint16(list_id), uint16(priority_val), title, description))

            case 2:
                // see current tasks
                // I haven't implemented priority filtering on the server yet, so these inputs won't have an effect, but they are still part of the message
                // need
                // min priority
                // max priority
                min_priority := prompt_for_int("\nWhat is the minimum priority value of task that should be returned? ", 0, 60000)
                max_priority := prompt_for_int("\nWhat is the maximum priority value of task that should be returned? ", 0, 60000)
                xmit(connection, ptmp.Prep_Query_Tasks(uint16(min_priority), uint16(max_priority)))
            case 3:
                // mark a task completed
                // just need to know what task ID to mark
                list_id := prompt_for_int("\nWhat list does the task belong to? ", 0, 255)
                task_id := prompt_for_int("\nTask ID to mark completed: ", 0, 60000)
                xmit(connection, ptmp.Prep_Mark_Task_Completed(uint16(list_id), uint16(task_id)))
            case 4:
                // remove a task
                permit_incomplete := 1 == prompt_for_int("\nShould incomplete tasks be allowed to be removed? (1 for yes, 0 for no) ", 0, 1)
                list_id := prompt_for_int("\nList ID to remove task from: ", 0, 255)
                task_id := prompt_for_int("\nTask ID to remove: ", 0, 60000) // I'm only allowing one at a time here, but the message allows for multiple tasks to be removed from the list
                xmit(connection, ptmp.Prep_Remove_Tasks(permit_incomplete, uint16(list_id), []uint16{uint16(task_id)}))
            case 5:
                // quit
                await_server := 1 == prompt_for_int("\nShould we wait for a server response before shutting down? (0 for no, 1 for yes) ", 0, 1)
                xmit(connection, ptmp.Prep_Close_Connection(await_server))
                quit_program = true
            default:
                fmt.Println("It shouldn't have been possible for you to get here...")
        }

    }

}

func main() {
    readConfig()
    connection, _ = connect_to_server()//HOST)


    if demo_mode {
        // This was how I was testing the protocol portion of the assignment before getting to the user input parsing
        req_conn := ptmp.Prep_Request_Connection("Ed Ucational", "p@55w0rd", 42, []uint16{1}, []uint16{})

        for false == connection_established {
            // keep trying to connect until it's established
            xmit(connection, req_conn)
        }

        // send some tasks to the server for it to keep track of
        new_task := ptmp.Prep_Create_New_Task(1, 1000, "Grade this assignment", "You should give Alec an A for doing such an awesome job with this project!")
        xmit(connection, new_task)

        new_task = ptmp.Prep_Create_New_Task(2, 1000, "Reject this!", "This is specifying a list that doesn't exist, so it should get rejected.")
        xmit(connection, new_task)

        new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be another task", "This is the second successful task, I hope.")
        xmit(connection, new_task)


        new_task = ptmp.Prep_Create_New_Task(1, 1000, "Be yet another task", "This is the third successful task, I hope.")
        xmit(connection, new_task)

        // prep a message to query the server about the tasks that it has stored
        querier := ptmp.Prep_Query_Tasks(0, 50000)
        xmit(connection, querier) // should show three tasks stored at this point
        xmit(connection, ptmp.Prep_Mark_Task_Completed(1, 1)) // set the second task (Be another task) to completed
        xmit(connection, querier) // query again, and we should see three tasks with the second one showing its status as completed
        xmit(connection, ptmp.Prep_Remove_Tasks(true, uint16(1), []uint16{2})) // remove that completed task from the list
        xmit(connection, querier) // we should now only see two tasks in the list that's returned
        xmit(connection, ptmp.Prep_Close_Connection(false)) // tell the server we're done
    } else {
        read_input()
    }

    connection.Close()

}
