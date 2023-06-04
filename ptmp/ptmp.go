package ptmp

import (
    "fmt"
    "encoding/gob"
    "bytes"
    "reflect"
)
var PRINTING_ENABLED bool = false

// probably want to use "bytes" for nearly everything
const (
    // MESSAGE TYPES
    // 0 Series - connection management
    REQUEST_CONNECTION byte = 0
    CONNECTION_RULES byte = 1
    CLOSE_CONNECTION byte = 2
    ACKNOWLEDGMENT byte = 3

    // 10 Series - list management
    CREATE_NEW_LIST byte = 10
    LIST_INFORMATION byte = 11
    QUERY_LISTS byte = 12
    REMOVE_LIST byte = 13


    // 20 Series - individual task management
    CREATE_NEW_TASK byte = 20
    TASK_INFORMATION byte = 21
    QUERY_TASKS byte = 22
    REMOVE_TASK byte = 24
    MARK_TASK_COMPLETED byte = 26
    

    // RESPONSE CODES
    // 200 series - postive definite
    SINGULAR_MSG_SUCCESS uint16 = 200
    MSG_SERIES_SUCCESS uint16 = 201

    // 300 series - positive indefinite
    CONDITIONAL_SUCCESS uint16 = 300

    // 400 series - negative indefinite
    UNABLE_TO_COMPLY uint16 = 400
    LIST_DOES_NOT_EXIST uint16 = 401
    TASK_DOES_NOT_EXIST uint16 = 402
    TIMEOUT_WARNING_ADDITIONAL_MSGS uint16 = 403
    TIMEOUT_WARNING_INACTIVE uint16 = 404
    CONDITIONAL_ORDER_FAILURE uint16 = 405
    INVALID_NAME uint16 = 406
    TEAPOT uint16 = 418


    // 500 series - negative definite
    SYNTAX_ERROR uint16 = 500
    PROTOCOL_VERSIONS_INCOMPATIBLE uint16 = 501
    MSG_NOT_IMPLEMENTED uint16 = 502
    MSG_CONTEXT_INVALID uint16 = 503


    MAX_PAYLOAD_SIZE uint16 = 1024
    USERNAME_SIZE uint16 = 32
    PASSWORD_SIZE uint16 = 32
    TITLE_MAX_LENGTH uint16 = 255
    DESCRIPTION_MAX_LENGTH uint16 = 511

    CURR_PROTOCOL_VERSION  byte = 1
)

type PTMP_Header struct {
    Protocol_Version byte
    Msg_Type_ID byte
    Msgs_To_Follow byte
    Payload_Byte_Length uint16
}

type Request_Connection struct {
    Username [USERNAME_SIZE]byte // apparently go doesn't do chars, and I don't think I can set a fixed-length string
    Password [PASSWORD_SIZE]byte 
    Timeout_Rule_Request uint16
    Client_Number_Versions_Supported uint16
    Client_Protocol_Versions_Supported []uint16
    Number_Extensions_Supported uint16
    Extensions_Supported []uint16
}

type Connection_Rules struct {
    Username_Ok byte
    Password_Ok byte
    Protocol_Version_To_Use uint16
    Number_Acceptable_Exts uint16
    Acceptable_Exts []uint16
}

type Acknowledgment struct {
    Response_Code uint16
    ID_Responding_To byte
}

type Close_Connection struct {
    Will_Await_Ack byte
}

type Create_New_Task struct {
    Associated_List_ID uint16
    Priority_Value uint16
    Length_of_Title byte // permit 1 to 255
    Task_Title []byte
    Length_of_Description uint16 // permit 1 to 511
    Task_Description []byte
}

type T_Inf struct {
    Task_Reference_Number uint16
    Task_Priority_Value uint16
    Length_of_Title byte // permit 1 to 255
    Task_Title []byte
    Description_Length uint16 // permit 1 to 511
    Task_Description []byte
    Completion_Status byte
}

type Task_Information struct {
    Number_of_Tasks uint16
    Task_Infos []T_Inf
}

type Query_Tasks struct {
    Minimum_Priority uint16
    Maximum_Priority uint16
}

type Remove_Tasks struct {
    Permit_Remove_Incomplete byte
    List_ID uint16
    Num_Tasks_Remove uint16
    Tasks_To_Remove []uint16
}

type Mark_Task_Completed struct {
    List_ID uint16
    Task_To_Mark uint16
}

func GetFixedBytes(b *bytes.Buffer, required_size uint16) [MAX_PAYLOAD_SIZE]byte {
    // Copy the contents of the buffer into a fixed-size byte array that
    // can then be put into the Pld slot of a PTMP_Msg
    out_arr := [MAX_PAYLOAD_SIZE]byte{}
    temp_bytes := b.Bytes()
//     fmt.Printf("Size of out_arr = %d, and size of temp_bytes = %d\n", len(out_arr), len(temp_bytes))
    for ii := 0; ii < int(MAX_PAYLOAD_SIZE) ; ii++ {
        if ii >= len(temp_bytes){
            break
        }
        out_arr[ii] = temp_bytes[ii]
    }
    
    return out_arr
}

type PAYLOADS interface {
    Request_Connection |
    Connection_Rules |
    Acknowledgment |
    Close_Connection |
    Create_New_Task |
    Task_Information |
    Query_Tasks |
    Remove_Tasks |
    Mark_Task_Completed
}

type STR_ARRAYS interface {
    [USERNAME_SIZE]byte |
    [TITLE_MAX_LENGTH]byte |
    [DESCRIPTION_MAX_LENGTH]byte
}

func EncodePayload[V PAYLOADS](msg V) [MAX_PAYLOAD_SIZE]byte {
    buff_temp := bytes.Buffer{}
    data_encoder := gob.NewEncoder(&buff_temp)
    data_encoder.Encode(msg)
    return GetFixedBytes(&buff_temp, MAX_PAYLOAD_SIZE)
}

func DecodePayload[V PAYLOADS](bytes_in [MAX_PAYLOAD_SIZE]byte) *V {
    new_arr := make([]byte, MAX_PAYLOAD_SIZE)
//     fmt.Printf("Bytes fed into the decoder: %+v\n",bytes_in)
    copy(new_arr, bytes_in[:MAX_PAYLOAD_SIZE])
//     fmt.Printf("The bytes in the copied array:%+v\n",new_arr)
    buff_temp := bytes.NewBuffer(new_arr)
    data_decoder := gob.NewDecoder(buff_temp)
    v := reflect.New(reflect.TypeOf((*V)(nil)).Elem())
//     output := &V{}
    data_decoder.Decode(v.Interface())

    return v.Interface().(*V)//output
}

func arrayify[X STR_ARRAYS](in_str string) X {
    temp_obj := reflect.New(reflect.TypeOf((*X)(nil)).Elem())
    out_arr := temp_obj.Interface().(*X)
    for ii := 0; ii < len(*out_arr); ii++ {
        if ii < len(in_str) {
            (*out_arr)[ii] = byte(in_str[ii])
        } else {
            (*out_arr)[ii] = 0
        }
    }
    return *out_arr
}

// All PTMP_Msgs consist of a common header and a payload converted into a plain byte-array.
type PTMP_Msg struct {
    Hdr PTMP_Header
    Pld [MAX_PAYLOAD_SIZE]byte
}

// Forces a string to be no longer than the specified length.
func trunc(inStr string, max_length uint16) string {
    if uint16(len(inStr)) <= max_length {
        return inStr
    }
    return inStr[:max_length]
}

func Bool2Byte(b_in bool) byte {
    // It is ridiculous that I need to make a function for this... how is it not legal to directly cast a bool to a byte?!
    if b_in {
        return byte(1)
    } else {
        return byte(0)
    }

}

func Byte2Bool(b_in byte) bool {
    return b_in != 0
}

// Encode the full PTMP_Msg into a byte array to go out over QUIC.
func EncodePacket(the_msg PTMP_Msg) []byte {

    buff_temp := bytes.Buffer{}
    data_encoder := gob.NewEncoder(&buff_temp)
    err_status := data_encoder.Encode(the_msg)//.Hdr)
    if err_status != nil {
        fmt.Printf("Encoder error: \n\t%+v\n",err_status)
    }

    return buff_temp.Bytes()
}

// Decode the raw byte-stream that is received over a connection so that the
// header may be parsed, thus allowing the payload of the message to be forwarded
// to the appopriate payload decoder function.
func DecodePacket(bytes_in []byte) *PTMP_Msg {
    temp_buff := bytes.NewBuffer(bytes_in)
    data_decoder := gob.NewDecoder(temp_buff)
    msg_out := &PTMP_Msg{}
    err_status := data_decoder.Decode(msg_out)
    if err_status != nil {
        fmt.Printf("Error when decoding:\n\t%+v\n\n",err_status)
    }
    return msg_out
}



// Assembles the generic header message for all PTMP_Msgs.
func prepHdr(msg_type byte, num_to_follow byte, payload_byte_length uint16) PTMP_Header {
    return PTMP_Header{
            Protocol_Version: CURR_PROTOCOL_VERSION,
            Msg_Type_ID: msg_type,
            Msgs_To_Follow: num_to_follow,
            Payload_Byte_Length: payload_byte_length,
        }
}



func Prep_Request_Connection(username string,
                             password string, 
                             timeout_request uint16, 
                             versions_supported []uint16,
                             extensions_supported []uint16) PTMP_Msg {
    req_conn := PTMP_Msg{}
    pld_size := USERNAME_SIZE + PASSWORD_SIZE + 6 + uint16(2*len(versions_supported) + 2*len(extensions_supported))
//     fmt.Printf("The payload size is %v.\n",pld_size)
    req_conn.Hdr = prepHdr(REQUEST_CONNECTION, 0, pld_size)
    pld := Request_Connection{
        Username: arrayify[[USERNAME_SIZE]byte](username),
        Password: arrayify[[PASSWORD_SIZE]byte](password),
        Timeout_Rule_Request: timeout_request,
        Client_Number_Versions_Supported: uint16(len(versions_supported)),
        Client_Protocol_Versions_Supported: versions_supported,
        Number_Extensions_Supported: uint16(len(extensions_supported)),
        Extensions_Supported: extensions_supported,
    }
    req_conn.Pld = EncodePayload(pld)//pld.ToBytes()

    return req_conn
}


func Prep_Connection_Rules(uname_ok bool,
                           pw_ok bool,
                           proto_ver uint16,
                           acceptable_exts []uint16) PTMP_Msg {
    conn_rules := PTMP_Msg{}
    pld_size := 6 + len(acceptable_exts) * 2
    conn_rules.Hdr = prepHdr(CONNECTION_RULES, 0, uint16(pld_size))
    pld := Connection_Rules{
                            Username_Ok: Bool2Byte(uname_ok),
                            Password_Ok: Bool2Byte(pw_ok),
                            Protocol_Version_To_Use: proto_ver,
                            Number_Acceptable_Exts: uint16(len(acceptable_exts)),
                            Acceptable_Exts: acceptable_exts,
                            }
    conn_rules.Pld = EncodePayload(pld)
    return conn_rules
}


func Prep_Acknowledgment(resp_code uint16,
                         msg_responding_to byte) PTMP_Msg{
    ack := PTMP_Msg{}
    pld_size := 3 // 1 uint16 + 1 byte = 3 bytes
    ack.Hdr = prepHdr(ACKNOWLEDGMENT, 0, uint16(pld_size))
    pld := Acknowledgment{
                          Response_Code: resp_code,
                          ID_Responding_To: msg_responding_to,
                          }
    ack.Pld = EncodePayload(pld)
    return ack
}

func Prep_Close_Connection(will_await bool) PTMP_Msg {
    close_conn := PTMP_Msg{}
    pld_size := 1
    close_conn.Hdr = prepHdr(CLOSE_CONNECTION, 0, uint16(pld_size))
    pld := Close_Connection{Will_Await_Ack : Bool2Byte(will_await)}
    close_conn.Pld = EncodePayload(pld)
    return close_conn
}

func Prep_Create_New_Task(list_id uint16,
                          priority uint16,
                          title string,
                          description string) PTMP_Msg {
    if len(title) < 1 || uint16(len(title)) > TITLE_MAX_LENGTH {
        panic(fmt.Errorf("Length of title of new task out of bounds [%v, %v].",1, TITLE_MAX_LENGTH))
    }
    if len(description) < 1 || uint16(len(description)) > DESCRIPTION_MAX_LENGTH {
        panic(fmt.Errorf("Length of title of new task out of bounds [%v, %v].",1, TITLE_MAX_LENGTH))
    }
    creator := PTMP_Msg{}
    pld_size := 2 + // list ID
                2 + // priority
                1 + // title length
                len(title) + // duh
                2 + // description length
                len(description) // duh
    creator.Hdr = prepHdr(CREATE_NEW_TASK, 0, uint16(pld_size))
    pld := Create_New_Task{
                            Associated_List_ID: list_id,
                            Priority_Value: priority,
                            Length_of_Title: byte(len(title)),
                            Task_Title: []byte(title),
                            Length_of_Description: uint16(len(description)),
                            Task_Description: []byte(description),
                          }
    creator.Pld = EncodePayload(pld)
    return creator
}

func Prep_Query_Tasks(min_priority uint16,
                      max_priority uint16) PTMP_Msg {
    query := PTMP_Msg{}
    pld_size := 4 // 2x uint16s
    query.Hdr = prepHdr(QUERY_TASKS, 0, uint16(pld_size))
    pld := Query_Tasks{
                        Maximum_Priority: max_priority,
                        Minimum_Priority: min_priority,
    }
    query.Pld = EncodePayload(pld)
    return query
}

func Prep_Task_Information(tasks []T_Inf, num_subsequent byte) PTMP_Msg {
    info := PTMP_Msg{}
    pld_size := uint16(len(tasks) * (2+2+1+2+1))
    for ii := 0; ii < len(tasks); ii++ {
        pld_size += uint16(tasks[ii].Length_of_Title) + tasks[ii].Description_Length
    }
    info.Hdr = prepHdr(TASK_INFORMATION, num_subsequent, pld_size)
    pld := Task_Information{
                            Number_of_Tasks: uint16(len(tasks)),
                            Task_Infos: tasks,
                            }
    info.Pld = EncodePayload(pld)
    return info
}

func Prep_Remove_Tasks(permit_incomplete bool,
                       listID uint16,
                       tasksToRemove []uint16) PTMP_Msg {
    rmTasks := PTMP_Msg{}
    pld_size := uint16(5 + len(tasksToRemove))
    rmTasks.Hdr = prepHdr(REMOVE_TASK, 0, pld_size)
    pld := Remove_Tasks{
                        Permit_Remove_Incomplete: Bool2Byte(permit_incomplete),
                        List_ID: listID,
                        Num_Tasks_Remove: uint16(len(tasksToRemove)),
                        Tasks_To_Remove: tasksToRemove,
                        }
    rmTasks.Pld = EncodePayload(pld)
    return rmTasks
}

func Prep_Mark_Task_Completed(listID uint16, taskID uint16) PTMP_Msg {
    completed := PTMP_Msg{}
    pld_size := uint16(4)
    completed.Hdr = prepHdr(MARK_TASK_COMPLETED, 0, pld_size)
    pld := Mark_Task_Completed{
                               List_ID: listID,
                               Task_To_Mark: taskID,
                              }
    completed.Pld = EncodePayload(pld)
    return completed
}
