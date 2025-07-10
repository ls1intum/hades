package log

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// publish log entries to NATS
func PublishLogsToNATS(nc *nats.Conn, buildJobLog Log) error {
	if nc == nil {
		slog.Error("Skipping log publish: nil NATS connection", slog.String("job_id", buildJobLog.JobID))
		return fmt.Errorf("nil NATS connection")
	}

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
