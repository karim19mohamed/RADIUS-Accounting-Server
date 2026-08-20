package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"encoding/json"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2866"
)

const (
	defaultAddr      = ":1813"
	defaultSharedKey = "testing123"
)

type Config struct {
	Addr        string
	SharedKey   string
	RedisAddr   string
	RedisDB     int
	LogFilePath string
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	cfg := Config{
		Addr:        getenv("RADIUS_ADDR", defaultAddr),
		SharedKey:   getenv("RADIUS_SECRET", defaultSharedKey),
		RedisAddr:   getenv("REDIS_ADDR", "redis:6379"),
		RedisDB:     0,
		LogFilePath: getenv("LOG_FILE", "/tmp/radius-server.log"),
	}

	log.Printf("Starting RADIUS accounting server on %s", cfg.Addr)
	log.Printf("Redis target: %s, DB %d", cfg.RedisAddr, cfg.RedisDB)

	conn, err := net.ListenPacket("udp", cfg.Addr)
	if err != nil {
		log.Fatalf("listen UDP: %v", err)
	}
	defer conn.Close()

	for {
		buf := make([]byte, 4096)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("read from socket: %v", err)
			continue
		}
		go handlePacket(conn, addr, buf[:n], cfg)
	}
}

func handlePacket(conn net.PacketConn, addr net.Addr, raw []byte, cfg Config) {
	// Verify the request authenticity using the shared secret before parsing.
	if !radius.IsAuthenticRequest(raw, []byte(cfg.SharedKey)) {
		log.Printf("unauthenticated RADIUS packet received from %s", addr.String())
		return
	}

	// Parse the packet into a structured form. Provide the shared secret so
	// any attributes that require the secret can be decoded by the library.
	packet, err := radius.Parse(raw, []byte(cfg.SharedKey))
	if err != nil {
		log.Printf("failed to parse RADIUS packet from %s: %v", addr.String(), err)
		return
	}

	if packet.Code != radius.CodeAccountingRequest {
		log.Printf("received non-accounting packet from %s: %v", addr.String(), packet.Code)
		return
	}

	// Extract attributes using the rfc helpers. Some return net.IP values.
	user := rfc2865.UserName_GetString(packet)
	acctStatus := rfc2866.AcctStatusType_Get(packet)
	acctSession := rfc2866.AcctSessionID_GetString(packet)
	nasIP := rfc2865.NASIPAddress_Get(packet)
	nasPort := rfc2865.NASPort_Get(packet)
	framedIP := rfc2865.FramedIPAddress_Get(packet)
	callingStation := rfc2865.CallingStationID_GetString(packet)
	calledStation := rfc2865.CalledStationID_GetString(packet)

	// Derive client IP string (strip port when possible).
	clientIP := addr.String()
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		clientIP = udpAddr.IP.String()
	}

	// Build an accounting record structure matching assignment fields.
	record := map[string]interface{}{
		"username":          user,
		"nas_ip_address":    func() string { if nasIP != nil { return nasIP.String() }; return "" }(),
		"nas_port":          int(nasPort),
		"acct_status_type":  acctStatus.String(),
		"acct_session_id":   acctSession,
		"framed_ip_address": func() string { if framedIP != nil { return framedIP.String() }; return "" }(),
		"calling_station_id": callingStation,
		"called_station_id":  calledStation,
		"timestamp":         time.Now().UTC().Format(time.RFC3339Nano),
		"client_ip":         clientIP,
		"packet_type":       "Accounting-Request",
	}

	// Log the JSON-encoded record for now (storage to Redis will be added
	// in the next step).
	j, _ := json.Marshal(record)
	log.Printf("Accounting record: %s", string(j))

	// Build and send an Accounting-Response that mirrors the request
	// identifier/authenticator so the sender can match the response.
	response := packet.Response(radius.CodeAccountingResponse)
	respBytes, err := response.Encode()
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	if _, err := conn.WriteTo(respBytes, addr); err != nil {
		log.Printf("send accounting response to %s: %v", addr.String(), err)
		return
	}

	log.Printf("Accounting response sent to %s", addr.String())
	fmt.Fprintf(os.Stdout, "Received accounting request from %s\n", addr.String())
}
