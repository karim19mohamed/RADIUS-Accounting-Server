package main

import (
    "net"
    "strings"
    "testing"
    "time"

    "layeh.com/radius"
    "layeh.com/radius/rfc2865"
    "layeh.com/radius/rfc2866"
    "github.com/alicebob/miniredis/v2"
    redis "github.com/go-redis/redis/v8"
)

// Test end-to-end: build an Accounting-Request, send to a local listener that
// invokes handlePacket, then verify Redis received a stored key and a response
// was sent back to the client.
func TestAccountingRequestEndToEnd(t *testing.T) {
    // Start a miniredis instance.
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatalf("miniredis start: %v", err)
    }
    defer mr.Close()

    rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

    // Configure server parameters.
    cfg := Config{
        SharedKey: "testing123",
        Rdb:       rdb,
    }

    // Start UDP listener on loopback with ephemeral port.
    ln, err := net.ListenPacket("udp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("listen: %v", err)
    }
    defer ln.Close()

    // Run a goroutine that reads one packet and processes it.
    go func() {
        buf := make([]byte, 4096)
        n, addr, err := ln.ReadFrom(buf)
        if err != nil {
            return
        }
        handlePacket(ln, addr, buf[:n], cfg)
    }()

    // Build a RADIUS Accounting-Request packet.
    p := radius.New(radius.CodeAccountingRequest, []byte(cfg.SharedKey))
    if err := rfc2865.UserName_AddString(p, "testuser"); err != nil {
        t.Fatalf("add username: %v", err)
    }
    if err := rfc2866.AcctSessionID_AddString(p, "session123"); err != nil {
        t.Fatalf("add session: %v", err)
    }
    if err := rfc2866.AcctStatusType_Add(p, rfc2866.AcctStatusType_Value_Start); err != nil {
        t.Fatalf("add status: %v", err)
    }

    // Encode packet to wire format.
    b, err := p.Encode()
    if err != nil {
        t.Fatalf("encode packet: %v", err)
    }

    // Dial the listener and send the packet, then wait for a response.
    addr := ln.LocalAddr().String()
    conn, err := net.Dial("udp", addr)
    if err != nil {
        t.Fatalf("dial: %v", err)
    }
    defer conn.Close()

    if _, err := conn.Write(b); err != nil {
        t.Fatalf("write: %v", err)
    }

    // Set a deadline and expect the server to reply with an Accounting-Response.
    conn.SetReadDeadline(time.Now().Add(2 * time.Second))
    resp := make([]byte, 4096)
    if _, err := conn.Read(resp); err != nil {
        t.Fatalf("did not receive response: %v", err)
    }

    // Allow small delay for Redis write.
    time.Sleep(200 * time.Millisecond)

    // Verify Redis contains a key matching pattern.
    keys := mr.Keys()
    found := false
    for _, k := range keys {
        if strings.HasPrefix(k, "radius:acct:") {
            found = true
            break
        }
    }
    if !found {
        t.Fatalf("no radius:acct:* key found in redis; keys=%v", keys)
    }
}
