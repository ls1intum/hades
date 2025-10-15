package log

import (
	"bufio"
	"bytes"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	logs "github.com/ls1intum/hades/shared/buildlogs"
)

const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
)

type LogParser interface {
	ParseContainerLogs(containerID string, jobID string) (logs.Log, error)
}

type StdLogParser struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func NewStdLogParser(stdout, stderr *bytes.Buffer) LogParser {
	return &StdLogParser{
		stdout: stdout,
		stderr: stderr,
	}
}

// converts raw standard log streams into structured log entries
func (p *StdLogParser) ParseContainerLogs(containerID string, jobID string) (logs.Log, error) {
	buildJobLog := logs.Log{
		JobID:       jobID,
		ContainerID: containerID,
	}

	// Process stdout and stderr
	for _, stream := range []struct {
		buf        *bytes.Buffer
		streamType string
	}{
		{buf: p.stdout, streamType: StreamStdout},
		{buf: p.stderr, streamType: StreamStderr},
	} {
		if err := processStream(stream.buf, stream.streamType, &buildJobLog.Logs); err != nil {
			return buildJobLog, fmt.Errorf("processing %s: %w", stream.streamType, err)
		}
	}

	slog.Debug("Parsed container logs",
		slog.String("container_id", containerID),
		slog.Int("entries_count", len(buildJobLog.Logs)))

	return buildJobLog, nil
}

// handles a single log stream (stdout or stderr)
func processStream(buf *bytes.Buffer, streamType string, entries *[]logs.LogEntry) error {
	scanner := bufio.NewScanner(buf)
	// Collect in local slice
	newEntries := make([]logs.LogEntry, 0, 100) // Pre-allocate with estimated capacity

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogLine(line, streamType)
		newEntries = append(newEntries, entry)
	}

	*entries = append(*entries, newEntries...)
	return scanner.Err()
}

// parses a single log line into a structured LogEntry
func parseLogLine(line, stream string) logs.LogEntry {
	var timestamp time.Time
	message := line

	// Regex pattern matches structured logs with format: time="..." level="..." msg="..."
	// This handles application logs that embed their own timestamps and levels
	var re = regexp.MustCompile(`time="[^"]*".*level="[^"]*".*msg="[^"]*"`)
	var parts []string

	if re.MatchString(message) {
		// Split into 3 sections: container timestamp, application timestamp, level + msg
		// Container timestamps wont be used in favor of application timestamps
		parts = strings.SplitN(line, " ", 3)
		// replace unused container timestamp
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
		slog.Warn("time parse failed, using current time", slog.String("message:", message))
	}

	entry := logs.LogEntry{
		Timestamp:    timestamp,
		Message:      message,
		OutputStream: stream,
	}

	return entry
}
