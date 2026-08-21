package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	redis "github.com/go-redis/redis/v8"
)

const defaultLogFile = "/var/log/radius_updates.log"

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	logFile := getenv("LOG_FILE", defaultLogFile)

	log.Printf("Starting Redis subscriber for %s", redisAddr)
	log.Printf("Log file: %s", logFile)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0,
	})

	// Ensure the log file exists (create if needed) so users see it immediately.
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed to create or open log file %s: %v", logFile, err)
	}
	f.Close()

	// One-time scan: log existing radius:acct:* keys so late-starting subscriber
	// can capture items created before it started (this does not modify keys).
	if err := scanAndLogExistingKeys(rdb, logFile); err != nil {
		log.Printf("failed to scan existing keys: %v", err)
	}

	for {
		if err := subscribeAndLog(rdb, logFile); err != nil {
			log.Printf("subscriber failed: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}

// scanAndLogExistingKeys performs a non-destructive scan of keys matching
// "radius:acct:*" and writes a timestamped line for each found key.
func scanAndLogExistingKeys(rdb *redis.Client, logFile string) error {
	ctx := context.Background()
	var cursor uint64
	for {
		keys, cur, err := rdb.Scan(ctx, cursor, "radius:acct:*", 100).Result()
		if err != nil {
			return err
		}

		for _, k := range keys {
			line := fmt.Sprintf("%s - Startup existing key: %s\n",
				time.Now().Format("2006-01-02 15:04:05.000000"), k)

			if err := appendLineToFile(logFile, line); err != nil {
				return err
			}
			log.Print(line)
		}

		cursor = cur
		if cursor == 0 {
			break
		}
	}
	return nil
}

func appendLineToFile(logFile, line string) error {
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(line); err != nil {
		return err
	}
	return nil
}

func subscribeAndLog(rdb *redis.Client, logFile string) error {
	ctx := context.Background()
	pubsub := rdb.PSubscribe(ctx, "__keyspace@0__:radius:acct:*")
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			return err
		}

		if msg == nil {
			continue
		}

		key := strings.TrimPrefix(msg.Channel, "__keyspace@0__:")
		if key == "" || !strings.Contains(key, "radius:acct:") {
			continue
		}

		line := fmt.Sprintf("%s - Received update for key: %s\n",
			time.Now().Format("2006-01-02 15:04:05.000000"),
			key,
		)

		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		if _, err := file.WriteString(line); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}

		log.Print(line)
	}
}
