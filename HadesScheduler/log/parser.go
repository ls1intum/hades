package log

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
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
	JobID         string     `json:"job_id"`
	ContainerName string     `json:"container_id"`
	Logs          []LogEntry `json:"logs"`
}

// converts raw log streams into structured log entries
func ParseContainerLogs(stdout, stderr *bytes.Buffer, containerName string) (Log, error) {
	var buildJobLog Log
	buildJobLog.ContainerName = containerName

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
		slog.String("container_id", containerName),
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
	var timestamp time.Time
	message := line

	// Regex pattern matches structured logs with format: time="..." level="..." msg="..."
	// This handles application logs that embed their own timestamps and levels
	var re = regexp.MustCompile(`time=".*level=.*msg="`)
	var parts []string

	if re.MatchString(message) {
		// Split into 3 sections: container timestamp, application timestamp, level + msg
		// Container timestamps wont be used in favor of application timestamps
		parts = strings.SplitN(line, " ", 3)
		// replace unsused container timestamp
		parts[0] = strings.TrimSuffix(strings.TrimPrefix(parts[1], `time="`), `"`)
		message = parts[2]
	} else {
		// Handle simple container logs with just timestamp and message
		parts = strings.SplitN(line, " ", 2)

		if len(parts) == 1 {
			message = ""
		} else {
			message = parts[1]
		}
	}

	if ts, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
		timestamp = ts
	} else {
		// If timestamp parsing fails
		timestamp = time.Now()
		slog.Debug("time parse failed, using current time", slog.String("message:", message))
	}

	entry := LogEntry{
		Timestamp:    timestamp,
		Message:      message,
		OutputStream: stream,
	}

	return entry
}
