package log

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

type Publisher interface {
	PublishLog(buildJobLog logs.Log) error
	PublishJobStatus(status logs.JobStatus, jobID string) error
}

type NATSPublisher struct {
	nc *nats.Conn
	pd *logs.HadesLogProducer
}

type message struct {
	Timestamp time.Time
	Content   string // Log or job status
	Message   string // If any extra message is needed
}

func NewNATSPublisher(nc *nats.Conn) (*NATSPublisher, error) {
	pd, err := logs.NewHadesLogProducer(nc)
	if err != nil {
		return nil, fmt.Errorf("creating log producer: %w", err)
	}

	return &NATSPublisher{
		nc: nc,
		pd: pd,
	}, nil
}

func (np NATSPublisher) PublishJobStatus(status logs.JobStatus, jobID string) error {
	var subject = status.Subject()
	m := message{time.Now(), jobID, "is" + status.String()}
	data, err := json.Marshal(m)

	if err != nil {
		slog.Error("Failed to marshal status message", slog.String("job_id", jobID), slog.Any("error", err))
		return fmt.Errorf("marshalling status message: %w", err)
	}

	if err := np.nc.Publish(subject, data); err != nil {
		slog.Error("Failed to publish job status to NATS subject", slog.String("status", status.String()), slog.String("job_id", jobID), slog.Any("error", err))
		return fmt.Errorf("publishing job status to NATS subject: %w", err)
	}

	return nil
}

// publish log entries to NATS JetStream
func (np NATSPublisher) PublishLog(buildJobLog logs.Log) error {
	np.pd.PublishLog(buildJobLog)
	return nil
}
