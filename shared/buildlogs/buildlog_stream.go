package logs

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
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}

	consumerName := "log-processor"
	consumer, err := js.CreateOrUpdateConsumer(ctx, "HADES_JOB_LOGS", jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy, // Get all messages from beginning corresponding to MaxMsgs and MaxAge
	})
	if err != nil {
		slog.Error("Failed to create JetStream consumer", "error", err)
		return nil, err
	}

	slog.Info("Created JetStream consumer", "consumer", consumerName)

	return &HadesLogConsumer{
		natsConnection: nc,
		js:             js,
		consumer:       consumer,
	}, nil
}

func (hlc *HadesLogConsumer) ConsumeLog(ctx context.Context, handler func(Log)) error {
	// Start consuming messages
	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping log consumption")
			return ctx.Err()
		default:
			// Fetch one message at a time
			batch, err := hlc.consumer.FetchNoWait(1)
			if err != nil {
				slog.Error("Failed to fetch log message", "error", err)
				time.Sleep(1 * time.Second) // Brief pause on error
				continue
			}

			msgs := batch.Messages()
			msg, ok := <-msgs
			if !ok {
				// No messages available, sleep briefly
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Parse the log message
			var buildJobLog Log
			if err := json.Unmarshal(msg.Data(), &buildJobLog); err != nil {
				slog.Error("Failed to unmarshal log message", "error", err, "subject", msg.Subject)

				// NAK the message so it can be redelivered
				if nakErr := msg.Nak(); nakErr != nil {
					slog.Error("Failed to NAK message", "error", nakErr)
				}
				continue
			}

			slog.Debug("Processing log", "job_id", buildJobLog.JobID, "subject", msg.Subject)

			// Process the log with your handler
			handler(buildJobLog)

			// Acknowledge successful processing
			if err := msg.Ack(); err != nil {
				slog.Error("Failed to acknowledge log message", "error", err)
			}
		}
	}
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
