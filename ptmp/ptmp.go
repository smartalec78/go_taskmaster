package ptmp

import (
    "fmt"
    "encoding/gob"
    "bytes"
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

type ByteSized interface {
    ToBytes() [MAX_PAYLOAD_SIZE]byte
}

type Request_Connection struct {
    Username [USERNAME_SIZE]byte // apparently go doesn't do chars, so bytes it is I guess
    Password [PASSWORD_SIZE]byte 
    Timeout_Rule_Request uint16
    Client_Number_Versions_Supported uint16
    Client_Protocol_Versions_Supported []uint16
    Number_Extensions_Supported uint16
    Extensions_Supported []uint16
    Padding []byte

}

func GetFixedBytes(b *bytes.Buffer, required_size uint16) [MAX_PAYLOAD_SIZE]byte {
    // Copy the contents of the buffer into a fixed-size byte array that
    // can then be put into the Pld slot of a PTMP_Msg
    out_arr := [MAX_PAYLOAD_SIZE]byte{}
    temp_bytes := b.Bytes()
//     fmt.Printf("Size of out_arr = %d, and size of temp_bytes = %d\n", len(out_arr), len(temp_bytes))
    for ii := 0; ii < int(MAX_PAYLOAD_SIZE) ; ii++ {
        out_arr[ii] = temp_bytes[ii]
    }
    
    return out_arr
}

func (RC Request_Connection) ToBytes() [MAX_PAYLOAD_SIZE]byte {
    buff_temp := bytes.Buffer{}
    data_encoder := gob.NewEncoder(&buff_temp)
    data_encoder.Encode(RC)
    
    return GetFixedBytes(&buff_temp, MAX_PAYLOAD_SIZE)
}

func Decode_Request_Connection(bytes_in [MAX_PAYLOAD_SIZE]byte) *Request_Connection {
    new_arr := make([]byte, MAX_PAYLOAD_SIZE)
    copy(new_arr, bytes_in[:MAX_PAYLOAD_SIZE])
    buff_temp := bytes.NewBuffer(new_arr)
    data_decoder := gob.NewDecoder(buff_temp)
    rc_out := &Request_Connection{}
    data_decoder.Decode(rc_out)
    return rc_out
}


type PTMP_Msg struct {
    Hdr PTMP_Header
    Pld [MAX_PAYLOAD_SIZE]byte
}

func EncodePacket(the_msg PTMP_Msg) []byte {

    buff_temp := bytes.Buffer{}
    data_encoder := gob.NewEncoder(&buff_temp)
    data_encoder.Encode(the_msg)

    return buff_temp.Bytes()
}

func DecodePacket(bytes_in []byte) *PTMP_Msg {
    temp_buff := bytes.NewBuffer(bytes_in)
    data_decoder := gob.NewDecoder(temp_buff)
    msg_out := &PTMP_Msg{}
    data_decoder.Decode(msg_out)
    return msg_out
}




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
                             extensions_supported []uint16) PTMP_Msg {
    req_conn := PTMP_Msg{}
    pld_size := USERNAME_SIZE + PASSWORD_SIZE + 6 + uint16(2*len(versions_supported) + 2*len(extensions_supported))
    fmt.Printf("The payload size is %v.\n",pld_size)
    req_conn.Hdr = prepHdr(REQUEST_CONNECTION, 0, pld_size)
    pld := Request_Connection{
        Timeout_Rule_Request: timeout_request,
        Client_Number_Versions_Supported: uint16(len(versions_supported)),
        Client_Protocol_Versions_Supported: versions_supported,
        Number_Extensions_Supported: uint16(len(extensions_supported)),
        Extensions_Supported: extensions_supported,
        Padding: make_padding(MAX_PAYLOAD_SIZE - pld_size),
    }
    uname := [USERNAME_SIZE]byte{}
    for ii :=  0; ii < int(USERNAME_SIZE); ii++ {
        if ii < len(username) {
            uname[ii] = byte(username[ii])
        } else {
            uname[ii] = 0
        }
    }
    pld.Username = uname
    pw := [PASSWORD_SIZE]byte{}
    for ii := range pw {
        if ii < len(password) {
            pw[ii] = byte(password[ii])
        } else {
            pw[ii] = 0
        }
    }
    pld.Password = pw
    req_conn.Pld = pld.ToBytes()

    return req_conn
}

func Test() {
    fmt.Println("Entering PTMP test functions.")
    vers_supported := []uint16{}
    exts_supported := []uint16{}
    rc := Prep_Request_Connection("Alec", "not a password", 42, vers_supported, exts_supported)
    fmt.Println(rc)
    fmt.Println(rc.Hdr)
    fmt.Println(rc.Pld)
    return
}
