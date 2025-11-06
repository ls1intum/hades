package log

import (
	"context"
	"fmt"
	"log/slog"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

// Publisher defines the interface for publishing logs and job status updates
type Publisher interface {
	PublishLog(ctx context.Context, buildJobLog logs.Log) error
	PublishJobStatus(ctx context.Context, status logs.JobStatus, jobID string) error
}

// NATSPublisher implements Publisher using NATS and JetStream
type NATSPublisher struct {
	nc *nats.Conn
	pd *logs.HadesLogProducer
}

// NewNATSPublisher creates a new NATS-based publisher.
// Returns an error if the connection is nil or log producer creation fails.
func NewNATSPublisher(nc *nats.Conn) (*NATSPublisher, error) {
	if nc == nil {
		return nil, fmt.Errorf("nil NATS connection")
	}

	pd, err := logs.NewHadesLogProducer(nc)
	if err != nil {
		return nil, fmt.Errorf("creating log producer: %w", err)
	}

	return &NATSPublisher{
		nc: nc,
		pd: pd,
	}, nil
}

// PublishJobStatus publishes a job status change to NATS.
// The status is published to the subject "hades.jobstatus.{status}".
func (np *NATSPublisher) PublishJobStatus(ctx context.Context, status logs.JobStatus, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("empty job ID")
	}

	if !status.IsValid() {
		return fmt.Errorf("invalid job status: %s", status)
	}

	subject := status.Subject()
	data := []byte(jobID)

	if ctx == nil {
		ctx = context.Background()
	}

	if err := np.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("publishing job status %s for job %s: %w", status, jobID, err)
	}

	slog.Debug("Published job status",
		"job_id", jobID,
		"status", status,
		"subject", subject)

	return nil
}

// PublishLog publishes log entries to NATS JetStream.
func (np *NATSPublisher) PublishLog(ctx context.Context, buildJobLog logs.Log) error {
	if err := np.pd.PublishLog(ctx, buildJobLog); err != nil {
		return fmt.Errorf("publishing job log: %w", err)
	}
	return nil
}
