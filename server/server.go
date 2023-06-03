package main

import (
    "context"
    "fmt"
    "github.com/quic-go/quic-go"
    "ajb497/ptmp"
)

const (
    BUFF_SIZE int = 2048
    HOSTNAME string = "localhost:20202"
)

var incoming_buff = make([]byte, BUFF_SIZE)

func setup_connection(hostname string) (quic.Connection, error) {
    fmt.Println("Server side is setting up.")
    server_listener, error_status := quic.ListenAddr(hostname, ptmp.GenerateTLSConfig(), nil)
    if error_status != nil {
        return nil, error_status
    } else {
        fmt.Println("Initial server setup steps are ok.")
    }
    return server_listener.Accept(context.Background())
}


func manage_incoming(server_connection quic.Connection) error {
    fmt.Println("In 'manage_incoming'!")
    server_stream, error_status := server_connection.AcceptStream(context.Background())
    if error_status != nil {
        fmt.Printf("Error on 'AcceptStream': \n\t%v\n",error_status)
        return error_status
    } else {
        fmt.Println("Got through 'AcceptStream'.")
    }
    fmt.Println("Connection established on the server side!")
    incoming_count, error_status := server_stream.Read(incoming_buff)
    if error_status != nil {
        fmt.Printf("Error on 'Read': \n\t%v\n",error_status)
        return error_status
    } else {
        fmt.Println("No error on 'Read'.")
    }

    fmt.Printf("Received %d bytes in!", incoming_count)
    msg_received := ptmp.DecodePacket(incoming_buff[0:incoming_count])
    fmt.Println(msg_received)
    connection_context := server_connection.Context()
    <-connection_context.Done()
    return nil

}

func main() {
    fmt.Println("Welcome to the PTMP server!")
    srvConn, err_status := setup_connection("localhost:20202")
    if err_status != nil {
        fmt.Printf("Error from connection attempt:\n\t%v\n", err_status)
        return
    } else {
        fmt.Println("Connection made, awaiting incoming traffic.")
    }
    err_status = manage_incoming(srvConn)
    if err_status != nil {
        fmt.Printf("Error from management of incoming:\n\t%v\n", err_status)
        return
    }
    fmt.Println("Done listening to incoming.")
}
