package log

import (
	"fmt"
	"log/slog"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	nc *nats.Conn
}

func NewNATSPublisher(nc *nats.Conn) *NATSPublisher {
	return &NATSPublisher{
		nc: nc,
	}
}

func (np NATSPublisher) PublishJobStatus(status string, jobID string) error {
	var subject = fmt.Sprintf("hades.status.%s", status)

	if err := np.nc.Publish(subject, []byte(jobID)); err != nil {
		slog.Error("Failed to publish job status to NATS subject", slog.String("status", status), slog.String("job_id", jobID), slog.Any("error", err))
		return fmt.Errorf("publishing job status to NATS subject: %w", err)
	}

	return nil
}

// publish log entries to NATS JetStream
func (np NATSPublisher) PublishLogs(buildJobLog logs.Log) error {
	producer, err := logs.NewHadesLogProducer(np.nc)

	if err != nil {
		slog.Error("failed to create JetStream stream for", slog.String("buildjob", buildJobLog.JobID), slog.Any("error", err))
	}
	producer.PublishLog(buildJobLog)

	return nil
}
