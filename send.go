package main

import (
    "fmt"
    "net"
    "time"

    "layeh.com/radius"
    "layeh.com/radius/rfc2865"
    "layeh.com/radius/rfc2866"
)

func main() {
    secret := []byte("testing123")
    p := radius.New(radius.CodeAccountingRequest, secret)
    _ = rfc2865.UserName_AddString(p, "testuser")
    _ = rfc2866.AcctSessionID_AddString(p, "session123")
    _ = rfc2866.AcctStatusType_Add(p, rfc2866.AcctStatusType_Value_Start)

    b, err := p.Encode()
    if err != nil {
        panic(err)
    }

    conn, err := net.Dial("udp", "127.0.0.1:1813")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    if _, err := conn.Write(b); err != nil {
        panic(err)
    }
    // wait for response
    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    resp := make([]byte, 4096)
    n, err := conn.Read(resp)
    if err != nil {
        fmt.Println("no response or read error:", err)
    } else {
        fmt.Println("received response bytes:", n)
    }
    // give server/subscriber a moment to write to Redis and logs
    time.Sleep(300 * time.Millisecond)
    fmt.Println("done")
}
