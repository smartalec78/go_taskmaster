package main

import (
    "context"
    "crypto/tls"
    "io"
    "log"
    "ajb497/ptmp"
    "github.com/quic-go/quic-go"
)

var buff_incoming = make([]byte, 2048)
const HOST string = "localhost:10101"


func connect_to_server(host string) (quic.Stream, error) {
    tls_conf := &tls.Config{
        InsecureSkipVerify: true,
        NextProtos:         []string{"quic-security-setup"},
    }
    conn2srv, err_status := quic.DialAddr(host, tls_conf, nil)
    if err_status != nil {
        return nil, err_status
    }

    return conn2srv.OpenStreamSync(context.Background())
}

func recv(stream quic.Stream) error {

    num_bytes_in, err_status := stream.Read(buff_incoming)
    if (err_status != nil) && (err_status != io.ErrUnexpectedEOF) {
        log.Printf("ERROR GETTING SERVER RESPONSE %+v", err_status)
        return err_status
    }

    pckt := ptmp.DecodePacket(buff_incoming[0:num_bytes_in])
    log.Printf("RECEIVED\n----------\n%+v\n----------", pckt)
    return nil
}

func xmit(stream quic.Stream, ptmp_out ptmp.PTMP_Msg) (int, error) {
    serialized := ptmp.EncodePacket(ptmp_out)
    num_bytes_out, err_status := stream.Write(serialized)
    if err_status != nil {
        log.Printf("Error writing to server: %+v", err_status)
        return 0, err_status
    }
    log.Printf("Just sent %d bytes to server\n%+v\n", num_bytes_out, serialized)
    return num_bytes_out, err_status
}

func main() {
    connection, _ := connect_to_server(HOST)
    req_conn := ptmp.Prep_Request_Connection("Alec", "not a password", 42, []uint16{1}, []uint16{})
    xmit(connection, req_conn)
    recv(connection)
	xmit(connection, req_conn)

    connection.Close()
    <-connection.Context().Done()
}
