package buildlogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// NatsLogSubject is the NATS subject pattern for job logs
	NatsLogSubject = "hades.logs.%s"
	// StreamName is the JetStream stream name for job logs
	StreamName = "HADES_JOB_LOGS"
	// Default batch sizes and timeouts
	defaultBatchSize     = 100
	defaultBatchTimeout  = 5 * time.Second
	defaultFetchSize     = 10
	defaultFetchWaitTime = 100 * time.Millisecond
	defaultShutdownTime  = 5 * time.Second
)

var (
	// ErrNilConnection is returned when NATS connection is nil
	ErrNilConnection = errors.New("nil NATS connection")
	// ErrNilJetStream is returned when JetStream context is nil
	ErrNilJetStream = errors.New("nil JetStream context")
	// ErrInvalidJobID is returned when job ID is empty or invalid
	ErrInvalidJobID = errors.New("invalid job ID")
)

// LogPublisher defines the interface for publishing logs
type LogPublisher interface {
	PublishJobLog(ctx context.Context, buildJobLog Log) error
}

// LogConsumer defines the interface for consuming logs
type LogConsumer interface {
	WatchJobLogs(ctx context.Context, jobID string, handler func(Log)) error
}

// HadesLogProducer handles publishing logs
type HadesLogProducer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
}

// HadesLogConsumer handles consuming logs
type HadesLogConsumer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
}

// NewHadesLogProducer creates a new log producer with JetStream stream setup.
// It creates or updates the HADES_JOB_LOGS stream with file storage and 24-hour retention.
// Returns an error if the NATS connection is nil or stream creation fails.
func NewHadesLogProducer(nc *nats.Conn) (*HadesLogProducer, error) {
	if nc == nil {
		return nil, ErrNilConnection
	}

	ctx := context.Background()

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	streamConfig := jetstream.StreamConfig{
		Name:       StreamName,
		Subjects:   []string{fmt.Sprintf(NatsLogSubject, "*")},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Duplicates: 1 * time.Minute,
		MaxMsgs:    10000,
		MaxAge:     24 * time.Hour,
	}

	stream, err := js.CreateOrUpdateStream(ctx, streamConfig)
	if err != nil {
		return nil, fmt.Errorf("creating JetStream stream: %w", err)
	}

	slog.Info("JetStream stream ready",
		"stream", stream.CachedInfo().Config.Name,
		"subjects", stream.CachedInfo().Config.Subjects)

	return &HadesLogProducer{
		natsConnection: nc,
		js:             js,
	}, nil
}

// NewHadesLogConsumer creates a new log consumer for reading logs from JetStream.
// Returns an error if the NATS connection is nil or JetStream context creation fails.
func NewHadesLogConsumer(nc *nats.Conn) (*HadesLogConsumer, error) {
	if nc == nil {
		return nil, ErrNilConnection
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	return &HadesLogConsumer{
		natsConnection: nc,
		js:             js,
	}, nil
}

// PublishLog publishes a log entry to JetStream for the specified job.
// The log is published to the subject "hades.logs.{jobID}".
// Returns an error if JetStream is nil, the log is invalid, or publishing fails.
func (hlp *HadesLogProducer) PublishLog(ctx context.Context, buildJobLog Log) error {
	if hlp.js == nil {
		return ErrNilJetStream
	}

	if buildJobLog.JobID == "" {
		return ErrInvalidJobID
	}

	subject := fmt.Sprintf(NatsLogSubject, buildJobLog.JobID)
	data, err := json.Marshal(buildJobLog)
	if err != nil {
		return fmt.Errorf("marshaling log for job %s: %w", buildJobLog.JobID, err)
	}

	if ctx == nil {
		ctx = context.Background()
	}

	_, err = hlp.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publishing log to subject %s: %w", subject, err)
	}

	slog.Debug("Published log",
		"job_id", buildJobLog.JobID,
		"container_id", buildJobLog.ContainerID,
		"entries", len(buildJobLog.Logs))

	return nil
}

// WatchJobLogs subscribes to logs for a specific job and calls the handler for each batch.
// It creates a job-specific durable consumer and automatically batches log entries for efficiency.
// The consumer is automatically cleaned up when the context is cancelled.
// Returns an error if the job ID is invalid or consumer creation fails.
func (hlc *HadesLogConsumer) WatchJobLogs(ctx context.Context, jobID string, handler func(Log)) error {
	if jobID == "" {
		return ErrInvalidJobID
	}

	if hlc.js == nil {
		return ErrNilJetStream
	}

	subject := fmt.Sprintf(NatsLogSubject, jobID)
	consumerName := fmt.Sprintf("job-watcher-%s", jobID)

	consumerConfig := jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	consumer, err := hlc.js.CreateOrUpdateConsumer(ctx, StreamName, consumerConfig)
	if err != nil {
		return fmt.Errorf("creating consumer for job %s: %w", jobID, err)
	}

	// Cleanup consumer when done
	defer func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTime)
		defer cancel()

		if err := hlc.js.DeleteConsumer(deleteCtx, StreamName, consumerName); err != nil {
			slog.Warn("Failed to delete job consumer",
				"job_id", jobID,
				"consumer", consumerName,
				"error", err)
		} else {
			slog.Debug("Deleted job consumer",
				"job_id", jobID,
				"consumer", consumerName)
		}
	}()

	slog.Info("Started watching job logs",
		"job_id", jobID,
		"subject", subject,
		"consumer", consumerName)

	return hlc.processBatchedLogs(ctx, consumer, jobID, handler)
}

// processBatchedLogs handles the batching and processing of log messages from the consumer.
// It batches log entries for efficiency and calls the handler periodically.
func (hlc *HadesLogConsumer) processBatchedLogs(ctx context.Context, consumer jetstream.Consumer, jobID string, handler func(Log)) error {
	const (
		batchSize    = defaultBatchSize
		batchTimeout = defaultBatchTimeout
		fetchSize    = defaultFetchSize
		fetchWait    = defaultFetchWaitTime
	)

	logBatch := make([]LogEntry, 0, batchSize)
	batchTimer := time.NewTimer(batchTimeout)
	defer batchTimer.Stop()

	flushBatch := func() {
		if len(logBatch) > 0 {
			handler(Log{JobID: jobID, Logs: logBatch})
			logBatch = logBatch[:0] // Reset batch
		}
		batchTimer.Reset(batchTimeout)
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch() // Flush remaining logs before stopping
			return ctx.Err()

		case <-batchTimer.C:
			flushBatch()

		default:
			batch, err := consumer.FetchNoWait(fetchSize)
			if err != nil {
				time.Sleep(fetchWait)
				continue
			}

			hasMessages := false
			for msg := range batch.Messages() {
				hasMessages = true

				var log Log
				if err := json.Unmarshal(msg.Data(), &log); err != nil {
					slog.Warn("Failed to unmarshal log message",
						"job_id", jobID,
						"error", err)
					if ackErr := msg.Nak(); ackErr != nil {
						slog.Warn("Failed to NAK message", "error", ackErr)
					}
					continue
				}

				logBatch = append(logBatch, log.Logs...)

				if ackErr := msg.Ack(); ackErr != nil {
					slog.Warn("Failed to ACK message", "error", ackErr)
				}

				// Flush batch if it gets too large
				if len(logBatch) >= batchSize {
					flushBatch()
				}
			}

			if !hasMessages {
				time.Sleep(fetchWait)
			}
		}
	}
}
