package main

import (
    "context"
    "io"
    "log"
    "ajb497/ptmp"
    "github.com/quic-go/quic-go"
)

var buff_incoming = make([]byte, 2048)
const HOST string = "localhost:10101"
const LOGGING_ENABLED bool = false

var active_proto_version uint16 = 1
var exts_enabled = make([]uint16, 0)
var proto_versions_supported = make([]uint16, 1)
var timeout_permitted uint16 = 30
var rcvdMsg *ptmp.PTMP_Msg
var connectionEstablished bool = false
var qconn quic.Connection

func connect_to_client(addr string) (quic.Connection, error) {
    if LOGGING_ENABLED {
        log.Printf("Server is initializing")
    }
    listener, err := quic.ListenAddr(addr, ptmp.GenerateTLSConfig(), nil)
    if err != nil {
        return nil, err
    }
    if LOGGING_ENABLED {
        log.Printf("Server finalizing setup.")
    }
    return listener.Accept(context.Background())
}

func recv() error {
    if LOGGING_ENABLED {
        log.Printf("Server is standing-by for a connection.")
    }
    stream, err_status := qconn.AcceptStream(context.Background())
    if err_status != nil {
        return err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Connection established.")
    }
    num_bytes_in, err_status := stream.Read(buff_incoming)
    if LOGGING_ENABLED {
        log.Printf("Read-in data from the stream")
    }
    if (err_status != nil) && (err_status!= io.ErrUnexpectedEOF) {
        log.Printf("Error attempting to read from client:\n\t%+v\n", err_status)
        return err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Server has receved %d bytes from the client.", num_bytes_in)
    }
    rcvdMsg = ptmp.DecodePacket(buff_incoming[0:num_bytes_in])
    if LOGGING_ENABLED {
        log.Printf("RECEIVED\n----------\n%+v\n----------", rcvdMsg)
    }

    determine_response()

    //Now lets convert into bytes
    serialized := ptmp.EncodePacket(ptmp.Prep_Request_Connection("Other User", "also, not a password", 47, []uint16{1}, []uint16{}))

    //Send to the server
    num_bytes_out, err_status := stream.Write(serialized)
    if err_status != nil {
        log.Printf("Error writing to client: %+v", err_status)
        return err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Just wrote %d bytes to client\n%+v\n", num_bytes_out, serialized)
    }
    //This is a stream so we need to have a way for the client to have the opportunity to receive
    //the message and then close the connection, we use context for this
    connCtx := qconn.Context()
    <-connCtx.Done()

    return nil
}


// func xmit(packet_out []byte) int, error {
// 	num_bytes_out, err_status :=
// }

func determine_response() {
	switch rcvdMsg.Hdr.Msg_Type_ID {
		case ptmp.REQUEST_CONNECTION:
			if !connectionEstablished {
				sendErrMsg(ptmp.MSG_CONTEXT_INVALID)
			} else {
				sendConnRules()
			}
		default:
			sendErrMsg(ptmp.MSG_NOT_IMPLEMENTED)
	}
}

func sendErrMsg(response_code uint16) {
}

func sendConnRules() {
// 	conn_rules := ptmp.EncodePacket(ptmp.Prep_Connection_Rules(true, true, active_proto_version, []uint16{}))
}

func main() {
	proto_versions_supported[0] = active_proto_version
	var err error
    qconn, err = connect_to_client(HOST)
    log.Printf("Server just initialized, error is %+v", err)
    recv()

    //time.Sleep(time.Second * 2)
}
