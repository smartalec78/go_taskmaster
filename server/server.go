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
const LOGGING_ENABLED bool = true

var active_proto_version uint16 = 1
var exts_enabled = make([]uint16, 0)
var proto_versions_supported = make([]uint16, 1)
var timeout_permitted uint16 = 30
var rcvdMsg *ptmp.PTMP_Msg
var connectionEstablished bool = false
var qconn quic.Connection
var stream quic.Stream

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
    var err_status error
    stream, err_status = qconn.AcceptStream(context.Background())
    
    if err_status != nil {
        return err_status
    }
    if LOGGING_ENABLED {
        log.Printf("Connection established.")
    }
    // Continuously look for incoming messages.
    for true {
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
    }

    connCtx := qconn.Context()
    <-connCtx.Done()

    return nil
}


func xmit(packet_out []byte) (int, error) {
    if LOGGING_ENABLED {
        log.Printf("Trying to transmit from the server:\n\tStream: \n\t\t%+v\n\tPacket:\n\n\t%+v\n\n",stream, packet_out)
    }
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

func determine_response() {
	switch rcvdMsg.Hdr.Msg_Type_ID {
		case ptmp.REQUEST_CONNECTION:
			if connectionEstablished {
				sendErrMsg(ptmp.MSG_CONTEXT_INVALID)
			} else {
				sendConnRules()
                connectionEstablished = true
			}
		default:
			sendErrMsg(ptmp.MSG_NOT_IMPLEMENTED)
	}
}

func sendErrMsg(response_code uint16) {
}

func sendConnRules() {
    conn_rules := ptmp.Prep_Connection_Rules(true, true, active_proto_version, []uint16{})
    encoded := ptmp.EncodePacket(conn_rules)
    if LOGGING_ENABLED {

        log.Printf("The Connection Rules message is \n\t%+v\n and encoded as \n\t%+v\n", conn_rules, encoded)
    }
	xmit(encoded)
}

func main() {
	proto_versions_supported[0] = active_proto_version
	var err error
    qconn, err = connect_to_client(HOST)
    log.Printf("Server just initialized, error is %+v", err)
    recv()

    //time.Sleep(time.Second * 2)
}
