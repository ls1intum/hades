package buildlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const NatsLogSubject = "hades.logs"

type HadesLogProducer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
}

type HadesLogConsumer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
	consumer       jetstream.Consumer
}

// creates a JetStream connection for persistent log delivery
func NewHadesLogProducer(nc *nats.Conn) (*HadesLogProducer, error) {
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}

	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       "HADES_JOB_LOGS",
		Subjects:   []string{fmt.Sprintf("%s.*", NatsLogSubject)},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		Duplicates: 1 * time.Minute, // Disallow duplicates for 1 minute
		MaxMsgs:    10000,
		MaxAge:     24 * time.Hour, // Retain logs for 24 hours by default
	})
	if err != nil {
		slog.Error("Failed to create JetStream stream", "error", err)
		return nil, err
	}
	slog.Info("Created JetStream stream", "stream", s)

	return &HadesLogProducer{
		natsConnection: nc,
		js:             js,
	}, nil
}

func NewHadesLogConsumer(nc *nats.Conn) (*HadesLogConsumer, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}

	return &HadesLogConsumer{
		natsConnection: nc,
		js:             js,
		consumer:       nil, // You can create a permanent consumer here if needed
	}, nil
}

func (hlp *HadesLogProducer) PublishLog(buildJobLog Log) error {
	if hlp.js == nil {
		slog.Error("Cannot publish logs: nil JetStream connection", slog.String("job_id", buildJobLog.JobID))
		return fmt.Errorf("nil JetStream connection")
	}

	subject := fmt.Sprintf("%s.%s", NatsLogSubject, buildJobLog.JobID) // "hades.logs.{jobID}"
	data, err := json.Marshal(buildJobLog)
	if err != nil {
		slog.Error("Failed to marshal log", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("marshalling log: %w", err)
	}

	_, err = hlp.js.Publish(context.Background(), subject, data)
	if err != nil {
		slog.Error("Failed to publish log to JetStream", slog.String("job_id", buildJobLog.JobID), slog.Any("error", err))
		return fmt.Errorf("publishing log to JetStream: %w", err)
	}

	return nil
}

// WatchJobLogs uses JetStream with job-specific consumer
func (hlc *HadesLogConsumer) WatchJobLogs(ctx context.Context, jobID string, handler func(Log)) error {
	subject := fmt.Sprintf("hades.logs.%s", jobID)
	consumerName := fmt.Sprintf("job-watcher-%s", jobID)

	// Create a temporary consumer for this specific job
	consumer, err := hlc.js.CreateOrUpdateConsumer(ctx, "HADES_JOB_LOGS", jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject, // Only consume messages from this job's subject
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("creating job consumer: %w", err)
	}

	// Cleanup consumer when done
	defer func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := hlc.js.DeleteConsumer(deleteCtx, "HADES_JOB_LOGS", consumerName); err != nil {
			slog.Error("Failed to delete job consumer", "job_id", jobID, "error", err)
		}
	}()

	logBatch := make([]LogEntry, 0, 100)         // Batch up to 100 log entries
	batchTimer := time.NewTimer(5 * time.Second) // Flush batch every 5 seconds
	defer batchTimer.Stop()

	slog.Info("Started watching logs for job", "job_id", jobID, "subject", subject)

	for {
		select {
		case <-ctx.Done():
			// Flush remaining logs before stopping
			if len(logBatch) > 0 {
				handler(Log{JobID: jobID, Logs: logBatch})
			}
			return ctx.Err()

		case <-batchTimer.C:
			// Flush batch on timer
			if len(logBatch) > 0 {
				handler(Log{JobID: jobID, Logs: logBatch})
				logBatch = logBatch[:0] // Reset batch
			}
			batchTimer.Reset(5 * time.Second)

		default:
			// Fetch messages
			batch, err := consumer.FetchNoWait(10) // Fetch up to 10 messages at once
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			msgs := batch.Messages()
			hasMessages := false

			for msg := range msgs {
				hasMessages = true
				var log Log
				if err := json.Unmarshal(msg.Data(), &log); err != nil {
					slog.Error("Failed to unmarshal log", "job_id", jobID, "error", err)
					msg.Nak()
					continue
				}

				// Add log entries to batch
				logBatch = append(logBatch, log.Logs...)
				msg.Ack()

				// Flush batch if it gets too large
				if len(logBatch) >= 100 {
					handler(Log{JobID: jobID, Logs: logBatch})
					logBatch = logBatch[:0] // Reset batch
					batchTimer.Reset(5 * time.Second)
				}
			}

			if !hasMessages {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
