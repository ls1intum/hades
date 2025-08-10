package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type ProcessedLog struct {
	JobID       string     `json:"job_id"`
	ContainerID string     `json:"container_id"`
	LogCount    int        `json:"log_count"`
	Logs        []LogEntry `json:"logs"`
}

func processLog(logMsg Log) {
	slog.Info("Process incoming log",
		slog.String("job_id", logMsg.JobID),
		slog.String("container_id", logMsg.ContainerID),
		slog.Int("log_entries", len(logMsg.Logs)),
	)

	// Basic validation
	if logMsg.JobID == "" {
		slog.Warn("Received log with empty job ID")
		return
	}

	// Transform to your DTO format
	processedLog := ProcessedLog{
		JobID:       logMsg.JobID,
		ContainerID: logMsg.ContainerID,
		LogCount:    len(logMsg.Logs),
		Logs:        logMsg.Logs,
	}

	// Send to endpoint
	if err := sendToEndpoint(processedLog); err != nil {
		slog.Error("Failed to send log to endpoint",
			slog.String("job_id", logMsg.JobID),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("Successfully processed and sent log",
		slog.String("job_id", logMsg.JobID),
	)
}

func sendToEndpoint(log ProcessedLog) error {
	// TODO: Replace with your actual endpoint URL
	endpoint := "http://your-endpoint-here/api/logs"

	jsonData, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("endpoint returned status: %d", resp.StatusCode)
	}

	return nil
}

// Optional: Add filtering logic if you only want certain jobs
func shouldProcessJob(jobID string) bool {
	// For now, process everything
	// Later you could add logic like:
	// - Only process certain job types
	// - Skip jobs that are already completed
	// - Filter based on some criteria
	return true
}

// Updated processLog with filtering:
func processLogWithFilter(logMsg Log) {
	if !shouldProcessJob(logMsg.JobID) {
		slog.Debug("Skipping job", slog.String("job_id", logMsg.JobID))
		return
	}

	processLog(logMsg)
}
