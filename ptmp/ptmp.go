package ptmp

import (
    "fmt"
)

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

    CURR_PROTOCOL_VERSION  byte = 1
)

type PTMP_Header struct {
    Protocol_Version byte
    Msg_Type_ID byte
    Msgs_To_Follow byte
    Payload_Byte_Length uint16
}

type Request_Connection_Payload struct {
    Username [USERNAME_SIZE]byte // apparently go doesn't do chars, so bytes it is I guess
    Password [PASSWORD_SIZE]byte 
    Timeout_Rule_Request uint16
    Client_Number_Versions_Supported uint16
    Client_Protocol_Versions_Supported []uint16
    Number_Extensions_Supported uint16
    Extensions_Supported []uint16
    Padding []byte

}

type PTMP_Msg struct {
    Hdr PTMP_Header
    Pld [MAX_PAYLOAD_SIZE]byte
}

type Request_Connection struct {
    PTMP_Msg
    PLD_RC Request_Connection_Payload
}
// type PTMP_Msg struct {
    // Hdr PTMP_Header
    // Payload [MAX_PAYLOAD_SIZE]byte
// }


func prepHdr(msg_type byte, num_to_follow byte, payload_byte_length uint16) PTMP_Header {
    return PTMP_Header{
            Protocol_Version: CURR_PROTOCOL_VERSION,
            Msg_Type_ID: msg_type,
            Msgs_To_Follow: num_to_follow,
            Payload_Byte_Length: payload_byte_length,
        }
}
func make_padding(num_bytes uint16) []byte {
    arrOut := make([]byte, num_bytes)

    return arrOut
}

func trunc(inStr string, max_length uint16) string {
    if uint16(len(inStr)) <= max_length {
        return inStr
    }
    return inStr[:max_length]
}


func Prep_Request_Connection(username string,
                             password string, 
                             timeout_request uint16, 
                             versions_supported []uint16,
                             extensions_supported []uint16) Request_Connection {
    req_conn := Request_Connection{}
    pld_size := USERNAME_SIZE + PASSWORD_SIZE + 6 + uint16(2*len(versions_supported) + 2*len(extensions_supported))
    req_conn.Hdr = prepHdr(REQUEST_CONNECTION, 0, pld_size)
    req_conn.PLD_RC = Request_Connection_Payload{
        Timeout_Rule_Request: timeout_request,
        Client_Number_Versions_Supported: uint16(len(versions_supported)),
        Client_Protocol_Versions_Supported: versions_supported,
        Number_Extensions_Supported: uint16(len(extensions_supported)),
        Extensions_Supported: extensions_supported,
        Padding: make_padding(MAX_PAYLOAD_SIZE - pld_size),
    }
    uname := [USERNAME_SIZE]byte{}
    for ii := range uname {
        if ii <= len(username) {
            uname[ii] = byte(username[ii])
        } else {
            uname[ii] = 0
        }
    }
    pw := [PASSWORD_SIZE]byte{}
    for ii := range pw {
        if ii <= len(password) {
            pw[ii] = byte(password[ii])
        } else {
            pw[ii] = 0
        }
    }
    req_conn.PLD_RC.Password = pw
    return req_conn
}

func Test() {
    fmt.Println("Entering PTMP test functions.")
    return
}
