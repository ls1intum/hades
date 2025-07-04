package log

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	StreamStdout     = "stdout"
	StreamStderr     = "stderr"
	LogSubjectFormat = "logs.%s"
)

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

// converts raw log streams into structured log entries
func ParseContainerLogs(stdout, stderr *bytes.Buffer, jobID, containerID string) (Log, error) {
	var buildJobLog Log

	buildJobLog.JobID = jobID
	buildJobLog.ContainerID = containerID

	// Process stdout and stderr
	for _, stream := range []struct {
		buf        *bytes.Buffer
		streamType string
	}{
		{buf: stdout, streamType: StreamStdout},
		{buf: stderr, streamType: StreamStderr},
	} {
		if err := processStream(stream.buf, stream.streamType, &buildJobLog.Logs); err != nil {
			return buildJobLog, fmt.Errorf("processing %s: %w", stream.streamType, err)
		}
	}

	slog.Debug("Parsed container logs",
		slog.String("container_id", containerID),
		slog.String("job_id", jobID),
		slog.Int("entries_count", len(buildJobLog.Logs)))

	return buildJobLog, nil
}

// handles a single log stream (stdout or stderr)
func processStream(buf *bytes.Buffer, streamType string, entries *[]LogEntry) error {
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogLine(line, streamType)
		*entries = append(*entries, entry)
	}
	return scanner.Err()
}

// parses a single log line into a structured LogEntry
func parseLogLine(line, stream string) LogEntry {
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
		Timestamp:    timestamp,
		Message:      message,
		OutputStream: stream,
	}

	return entry
}
