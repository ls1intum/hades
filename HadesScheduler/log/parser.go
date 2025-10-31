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
	// StreamStdout identifies stdout log stream
	StreamStdout = "stdout"
	// StreamStderr identifies stderr log stream
	StreamStderr = "stderr"
)

var (
	// logLineRegex matches structured logs with format: time="..." level="..." msg="..."
	logLineRegex = regexp.MustCompile(`time="[^"]*".*level="[^"]*".*msg="[^"]*"`)
)

// LogParser defines the interface for parsing container logs
type LogParser interface {
	ParseContainerLogs(containerID string, jobID string) (logs.Log, error)
}

// StdLogParser parses Docker container logs from stdout and stderr streams
type StdLogParser struct {
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// NewStdLogParser creates a new LogParser from stdout and stderr buffers.
// Both stdout and stderr must be non-nil.
func NewStdLogParser(stdout, stderr *bytes.Buffer) LogParser {
	if stdout == nil {
		stdout = &bytes.Buffer{}
	}
	if stderr == nil {
		stderr = &bytes.Buffer{}
	}

	return &StdLogParser{
		stdout: stdout,
		stderr: stderr,
	}
}

// ParseContainerLogs converts raw standard log streams into structured log entries.
// It processes both stdout and stderr, preserving timestamps and stream types.
func (p *StdLogParser) ParseContainerLogs(containerID string, jobID string) (logs.Log, error) {
	buildJobLog := logs.Log{
		JobID:       jobID,
		ContainerID: containerID,
		Logs:        make([]logs.LogEntry, 0),
	}

	streams := []struct {
		buf        *bytes.Buffer
		streamType string
	}{
		{buf: p.stdout, streamType: StreamStdout},
		{buf: p.stderr, streamType: StreamStderr},
	}

	for _, stream := range streams {
		if err := processStream(stream.buf, stream.streamType, &buildJobLog.Logs); err != nil {
			return buildJobLog, fmt.Errorf("processing %s: %w", stream.streamType, err)
		}
	}

	slog.Debug("Parsed container logs",
		"container_id", containerID,
		"job_id", jobID,
		"entries_count", len(buildJobLog.Logs))

	return buildJobLog, nil
}

// processStream handles a single log stream (stdout or stderr) and appends parsed entries
func processStream(buf *bytes.Buffer, streamType string, entries *[]logs.LogEntry) error {
	if buf == nil || buf.Len() == 0 {
		return nil
	}

	scanner := bufio.NewScanner(buf)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB buffer, 1MB max

	// Pre-allocate with estimated capacity
	newEntries := make([]logs.LogEntry, 0, 100)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogLine(line, streamType)
		newEntries = append(newEntries, entry)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning %s: %w", streamType, err)
	}

	*entries = append(*entries, newEntries...)
	return nil
}

// parseLogLine parses a single log line into a structured LogEntry.
// It handles both structured application logs and simple container logs.
func parseLogLine(line, stream string) logs.LogEntry {
	var timestamp time.Time
	message := line

	// Regex pattern matches structured logs with format: time="..." level="..." msg="..."
	if logLineRegex.MatchString(line) {
		// Handle structured application logs with embedded timestamps
		timestamp, message = parseStructuredLog(line)
	} else {
		// Handle simple container logs with timestamp prefix
		timestamp, message = parseSimpleLog(line)
	}

	return logs.LogEntry{
		Timestamp:    timestamp,
		Message:      message,
		OutputStream: stream,
	}
}

// parseStructuredLog extracts timestamp and message from structured logs
func parseStructuredLog(line string) (time.Time, string) {
	// Split: container timestamp | application time="..." | level + msg
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return time.Now(), line
	}

	// Extract application timestamp from time="..." format
	timeStr := strings.TrimSuffix(strings.TrimPrefix(parts[1], `time="`), `"`)
	timestamp, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		slog.Debug("Failed to parse structured log timestamp",
			"timestamp", timeStr,
			"error", err)
		timestamp = time.Now()
	}

	return timestamp, parts[2]
}

// parseSimpleLog extracts timestamp and message from simple container logs
func parseSimpleLog(line string) (time.Time, string) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 0 {
		return time.Now(), ""
	}

	timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		slog.Debug("Failed to parse timestamp, using current time",
			"timestamp", parts[0],
			"error", err)
		return time.Now(), line
	}

	message := ""
	if len(parts) > 1 {
		message = parts[1]
	}

	return timestamp, message
}
