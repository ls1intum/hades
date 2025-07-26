package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
)

type HadesAdapterConfig struct {
	NatsConfig utils.NatsConfig
	Topic      string
}

type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	OutputStream string    `json:"output_stream"`
}

type Log struct {
	JobID       string     `json:"job_id"`
	ContainerID string     `json:"container_id"`
	Logs        []LogEntry `json:"logs"`
}

// workflow
// 1. listen
// 2. filter and batch logs (probably requires an input of some sort from whoever wants the logs)
// 3. map to DTO
// 4. send to endpoint

func main() {
	var cfg HadesAdapterConfig

	// Connect to NATS server
	nc, err := nats.Connect(cfg.NatsConfig.URL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	log.Println("Connected to NATS server")

	// Subscribe to all log messages (using wildcard for now)
	sub, err := nc.Subscribe("logs.*", func(m *nats.Msg) {
		var logMsg Log

		// Unmarshal the JSON message back to Log struct
		if err := json.Unmarshal(m.Data, &logMsg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			return
		}

		// Process the received log
		log.Printf("Received log from job %s/n%s", logMsg.JobID, logMsg.Logs)

		// You can add your own processing logic here
		processLog(logMsg)
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	log.Println("Listening for log messages on 'logs.*'...")

	// Set up signal handling before we start waiting
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Block here waiting for signal
	sig := <-c
	log.Printf("Received signal %v, shutting down...", sig)

}

// processLog handles the received log message
func processLog(log Log) {
	// Add your custom processing logic here
	// Examples:
	// - Store to database
	// - Write to file
	// - Forward to another system

}
