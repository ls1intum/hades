package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/nats-io/nats.go"
)

const (
	StreamStdout     = "stdout"
	StreamStderr     = "stderr"
	LogSubjectFormat = "logs.%s"
)

type LogEntry struct {
	JobID       string    `json:"job_id"`
	ContainerID string    `json:"container_id"`
	Timestamp   time.Time `json:"timestamp"`
	Message     string    `json:"message"`
	Stream      string    `json:"stream"`
	Level       string    `json:"level,omitempty"`
}

// retrieves and demultiplexes container logs
func getContainerLogs(ctx context.Context, client *client.Client, containerID string) (*bytes.Buffer, *bytes.Buffer, error) {
	logReader, err := client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("getting container logs: %w", err)
	}
	defer logReader.Close()

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if _, err := stdcopy.StdCopy(stdout, stderr, logReader); err != nil {
		return nil, nil, fmt.Errorf("demultiplexing logs: %w", err)
	}

	return stdout, stderr, nil
}

// converts raw log streams into structured log entries
func parseContainerLogs(stdout, stderr *bytes.Buffer, jobID, containerID string) ([]LogEntry, error) {
	var entries []LogEntry

	// Process stdout and stderr
	for _, stream := range []struct {
		buf        *bytes.Buffer
		streamType string
	}{
		{buf: stdout, streamType: StreamStdout},
		{buf: stderr, streamType: StreamStderr},
	} {
		if err := processStream(stream.buf, stream.streamType, jobID, containerID, &entries); err != nil {
			return nil, fmt.Errorf("processing %s: %w", stream.streamType, err)
		}
	}

	slog.Debug("Parsed container logs",
		slog.String("container_id", containerID),
		slog.String("job_id", jobID),
		slog.Int("entries_count", len(entries)))

	return entries, nil
}

// handles a single log stream (stdout or stderr)
func processStream(buf *bytes.Buffer, streamType, jobID, containerID string, entries *[]LogEntry) error {
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogLine(line, jobID, containerID, streamType)
		*entries = append(*entries, entry)
	}
	return scanner.Err()
}

// parses a single log line into a structured LogEntry
func parseLogLine(line, jobID, containerID, stream string) LogEntry {
	timestamp := time.Now()
	message := line

	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			timestamp = ts
			message = parts[1]
		}
	}

	entry := LogEntry{
		JobID:       jobID,
		ContainerID: containerID,
		Timestamp:   timestamp,
		Message:     message,
		Stream:      stream,
	}

	if level := extractLogLevel(message); level != "" {
		entry.Level = level
	}

	return entry
}

// determines the log level from the message content
func extractLogLevel(message string) string {
	message = strings.ToLower(message)
	for _, level := range []struct {
		level    string
		keywords []string
	}{
		{"error", []string{"level=error", "error"}},
		{"warn", []string{"level=warn", "warning"}},
		{"info", []string{"level=info", "info"}},
		{"debug", []string{"level=debug", "debug"}},
	} {
		for _, keyword := range level.keywords {
			if strings.Contains(message, keyword) {
				return level.level
			}
		}
	}
	return ""
}

// writeContainerLogsToNATS sends container logs to NATS
func writeContainerLogsToNATS(ctx context.Context, client *client.Client, nc *nats.Conn, containerID, jobID string) error {
	stdout, stderr, err := getContainerLogs(ctx, client, containerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	entries, err := parseContainerLogs(stdout, stderr, jobID, containerID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	return publishLogsToNATS(nc, jobID, entries)
}

// publishLogsToNATS publishes log entries to NATS
func publishLogsToNATS(nc *nats.Conn, jobID string, entries []LogEntry) error {
	subject := fmt.Sprintf(LogSubjectFormat, jobID)

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			slog.Error("Failed to marshal log entry",
				slog.String("job_id", jobID),
				slog.Any("error", err))
			continue
		}

		if err := nc.Publish(subject, data); err != nil {
			slog.Error("Failed to publish log to NATS",
				slog.String("job_id", jobID),
				slog.Any("error", err))
		}
	}

	return nil
}

func copyFileToContainer(ctx context.Context, client *client.Client, containerID, srcPath, dstPath string) error {
	scriptFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer scriptFile.Close()

	// Create a buffer to hold the tar archive
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	// Read the script content
	scriptContent, err := io.ReadAll(scriptFile)
	if err != nil {
		return err
	}
	defer tw.Close()

	// Add the script content to the tar archive
	tarHeader := &tar.Header{
		Name: "script.sh",
		Size: int64(len(scriptContent)),
		Mode: 0755, // Make sure the script is executable
	}
	if err := tw.WriteHeader(tarHeader); err != nil {
		return err
	}
	if _, err := tw.Write(scriptContent); err != nil {
		return err
	}

	opts := container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		// CopyUIDGID: true,
	}

	err = client.CopyToContainer(ctx, containerID, dstPath, &buf, opts)
	if err != nil {
		slog.Error("Failed to copy script to container", slog.Any("error", err))
		return err
	}
	return nil
}
