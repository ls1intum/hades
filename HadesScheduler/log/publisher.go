package log

import (
	"encoding/json"
	"fmt"
	"log/slog"

	log "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

type Publisher interface {
	PublishLogs(buildJobLog log.Log) error
}

type NATSPublisher struct {
	nc *nats.Conn
}

func NewNATSPublisher(nc *nats.Conn) *NATSPublisher {
	return &NATSPublisher{
		nc: nc,
	}
}

// publish log entries to NATS
func (np NATSPublisher) PublishLogs(buildJobLog log.Log) error {
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

	return nil
}
