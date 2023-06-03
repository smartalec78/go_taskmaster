package main
import (
    "fmt"
    "ajb497/ptmp"
    "github.com/quic-go/quic-go"
    "crypto/tls"
    "context"
    "time"
)
// hard-coded host name used for this demonstration program
const (
    HOSTNAME string = "localhost:20202"
)

func setup_connection(hostname string) (quic.Stream, error) {
    tls_config := &tls.Config{
                              InsecureSkipVerify: true,
                              NextProtos: []string{"quic-security-setup"},
                          }
                                             // host to connect to, configuration of TLS for the connection, miscellaneous additional QUIC configuration items
    connection, error_status := quic.DialAddr(hostname, tls_config, nil)
    if error_status != nil {
        return nil, error_status
    }
    fmt.Println("Client-side connection configured!")
    return connection.OpenStreamSync(context.Background())
}

func send_msg(outstream quic.Stream, ptmp_out ptmp.PTMP_Msg) (int, error) {
//     outgoing_bytes := ptmp.EncodePacket(ptmp_out)
    num_out, err_status := outstream.Write([]byte{1,2,3,4, 0})//outgoing_bytes)
    if err_status != nil {
        return 0, err_status
    }
    return num_out, err_status
}

func main() {
    fmt.Println("Welcome to Alec's client for the PTMP.")
//     ptmp.Test()
    quicConnection, errStat := setup_connection("localhost:20202")
    if quicConnection == nil || errStat != nil {
        fmt.Printf("Connection unable to be established.  Error:\n\t%v\n",errStat)
    } else {
        fmt.Printf("I guess there wasn't an error in the connection setup?\n")
    }
    fmt.Printf("Tried to make a connection: %+v\n%+v", quicConnection, errStat)

    time.Sleep(time.Second*6)
    conn_req := ptmp.Prep_Request_Connection("Alec", "not a real password",
                                             42, []uint16{1}, []uint16{})

    b_sent, errStat := send_msg(quicConnection, conn_req)
    fmt.Printf("Bytes sent: %d, error status: %v\n",b_sent, errStat)
    quicConnection.Close()
    <-quicConnection.Context().Done()
    return
}
