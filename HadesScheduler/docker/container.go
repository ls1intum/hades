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
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	OutputStream string    `json:"output_stream"`
}

type Log struct {
	JobID       string     `json:"job_id"`
	ContainerID string     `json:"container_id"`
	Logs        []LogEntry `json:"logs"`
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
func parseContainerLogs(stdout, stderr *bytes.Buffer, jobID, containerID string) (Log, error) {
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

// writeContainerLogsToNATS sends container logs to NATS
func writeContainerLogsToNATS(ctx context.Context, client *client.Client, nc *nats.Conn, containerID, jobID string) error {
	stdout, stderr, err := getContainerLogs(ctx, client, containerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	buildJobLog, err := parseContainerLogs(stdout, stderr, jobID, containerID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	return publishLogsToNATS(nc, buildJobLog)
}

// publishLogsToNATS publishes log entries to NATS
func publishLogsToNATS(nc *nats.Conn, buildJobLog Log) error {
	subject := fmt.Sprintf(LogSubjectFormat, buildJobLog.JobID)

	data, err := json.Marshal(buildJobLog)
	if err != nil {
		slog.Error("Failed to marshal log", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("marshalling log: %w", err)
	}

	if err := nc.Publish(subject, data); err != nil {
		slog.Error("Failed to publish log to NATS", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("publishing log to NATS: %w", err)
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
