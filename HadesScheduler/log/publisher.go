package log

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

type Publisher interface {
	PublishLogs(buildJobLog Log) error
}

type NATSPublisher struct {
	nc *nats.Conn
}

func NewNATSPublisher(nc *nats.Conn) *NATSPublisher {
	return &NATSPublisher{
		nc: nc,
	}
}

func (np NATSPublisher) PublishLogs(buildJobLog Log) error {
	if np.nc == nil {
		slog.Error("Cannot publish logs: nil NATS connection", slog.String("job_id", buildJobLog.JobID))
		return fmt.Errorf("nil NATS connection")
	}

	subject := fmt.Sprintf(LogSubjectFormat, buildJobLog.JobID)
	data, err := json.Marshal(buildJobLog)

	if err != nil {
		slog.Error("Failed to marshal log", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("marshalling log: %w", err)
	}

	if err := np.nc.Publish(subject, data); err != nil {
		slog.Error("Failed to publish log to NATS", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("publishing log to NATS: %w", err)
	}
	slog.Debug("Log published to NATS", slog.String("job_id", buildJobLog.JobID), slog.String("subject", subject))

	return nil
}
