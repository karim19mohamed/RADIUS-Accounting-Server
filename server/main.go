package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
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
	packet, err := radius.Parse(raw, []byte(cfg.SharedKey))
	if err != nil {
		log.Printf("failed to parse RADIUS packet from %s: %v", addr.String(), err)
		return
	}

	if packet.Code != radius.CodeAccountingRequest {
		log.Printf("received non-accounting packet from %s: %v", addr.String(), packet.Code)
		return
	}

	user := rfc2865.UserName_GetString(packet)
	status := rfc2865.AcctStatusType_Get(packet)
	sessionID := rfc2865.AcctSessionId_GetString(packet)
	stationID := rfc2865.CallingStationId_GetString(packet)

	log.Printf("Accounting request received: user=%s status=%v session=%s caller=%s", user, status, sessionID, stationID)

	// Placeholder for parsing more attributes and storing them to Redis.
	_ = time.Now().Unix()
	response := radius.New(radius.CodeAccountingResponse, []byte(cfg.SharedKey))
	if _, err := conn.WriteTo(response.Bytes(), addr); err != nil {
		log.Printf("send accounting response to %s: %v", addr.String(), err)
		return
	}

	log.Printf("Accounting response sent to %s", addr.String())
	fmt.Fprintf(os.Stdout, "Received accounting request from %s\n", addr.String())
}
