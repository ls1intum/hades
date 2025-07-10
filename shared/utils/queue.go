package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const NatsSubject = "hades.jobs"

var priorities = []Priority{HighPriority, MediumPriority, LowPriority}

type HadesProducer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
	kv             jetstream.KeyValue
}

type HadesConsumer struct {
	natsConnection *nats.Conn
	concurrency    uint
	consumers      map[Priority]jetstream.Consumer
	kv             jetstream.KeyValue
}

// SetupNatsConnection creates a connection to NATS server
func SetupNatsConnection(config NatsConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("HadesAPI"),
		nats.Timeout(10 * time.Second),
		nats.ReconnectWait(5 * time.Second),
		nats.MaxReconnects(10),
	}

	// Add credentials if provided
	if config.Username != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.Username, config.Password))
	}

	// Add TLS if enabled
	if config.TLS {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12, // Ensure TLS 1.2 or higher
		}
		opts = append(opts, nats.Secure(tlsConfig))
	}

	// Connect to NATS
	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		return nil, err
	}

	slog.Info("Connected to NATS server", "url", config.URL)
	return nc, nil
}

// SetupNatsJetStream creates a JetStream connection for persistent message delivery
func NewHadesProducer(nc *nats.Conn) (*HadesProducer, error) {
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}

	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       "HADES_JOBS",
		Subjects:   []string{fmt.Sprintf("%s.*", NatsSubject)},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.WorkQueuePolicy,
		Duplicates: 1 * time.Minute, // Disallow duplicates for 1 minute
		MaxMsgs:    -1,
		MaxAge:     24 * time.Hour, // Retain jobs for 24 hours by default
	})
	if err != nil {
		slog.Error("Failed to create JetStream stream", "error", err)
		return nil, err
	}
	slog.Info("Created JetStream stream", "stream", s)

	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "HADES_JOBS",
	})
	if err != nil {
		slog.Error("Failed to create JetStream KeyValue store", "error", err)
		return nil, err
	}

	return &HadesProducer{
		natsConnection: nc,
		js:             js,
		kv:             kv,
	}, nil
}

func NewHadesConsumer(nc *nats.Conn, concurrency uint) (*HadesConsumer, error) {
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}
	consumers := make(map[Priority]jetstream.Consumer, len(priorities))

	// Create a consumer for each priority (one consumer is shared across all worker nodes)
	for _, priority := range priorities {
		consumerName := fmt.Sprintf("HADES_JOBS_%s", priority)
		cons, err := js.CreateOrUpdateConsumer(ctx, "HADES_JOBS", jetstream.ConsumerConfig{
			Name:          consumerName,
			Durable:       consumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			FilterSubject: PrioritySubject(priority),
		})
		if err != nil {
			slog.Error("Failed to create JetStream consumer", "error", err, "priority", priority)
			return nil, err
		}
		consumers[priority] = cons
		slog.Info("Created JetStream consumer", "consumer", consumerName, "priority", PrioritySubject(priority))
	}

	slog.Info("Created JetStream consumer", "consumers", consumers)

	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "HADES_JOBS",
	})
	if err != nil {
		slog.Error("Failed to create JetStream KeyValue store", "error", err)
		return nil, err
	}
	return &HadesConsumer{
		natsConnection: nc,
		consumers:      consumers,
		concurrency:    concurrency,
		kv:             kv,
	}, nil
}

func (hp HadesProducer) EnqueueMediumJob(ctx context.Context, queuePayloud payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayloud, MediumPriority)
}

func (hp HadesProducer) EnqueueHighJob(ctx context.Context, queuePayloud payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayloud, HighPriority)
}

func (hp HadesProducer) EnqueueLowJob(ctx context.Context, queuePayloud payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayloud, LowPriority)
}

func (hp HadesProducer) EnqueueJobWithPriority(ctx context.Context, queuePayloud payload.QueuePayload, priority Priority) error {
	bytesPayload, err := json.Marshal(queuePayloud)
	if err != nil {
		slog.Error("Failed to marshal payload", "error", err)
		return err
	}
	pubAckFuture, err := hp.js.PublishAsync(PrioritySubject(priority), queuePayloud.ID[:], jetstream.WithMsgID(queuePayloud.ID.String()))
	if err != nil {
		return err
	}

	// Wait (bounded) for the server ack
	if _, err := pubAckFuture.Ok(); err != nil {
		return err
	}

	// Wait (bounded) for the server ack
	if _, err := pubAckFuture.Ok(); err != nil {
		return err
	}

	_, err = hp.kv.Put(ctx, queuePayloud.ID.String(), bytesPayload)
	return err
}

func (hc HadesConsumer) DequeueJob(ctx context.Context, processing func(payload payload.QueuePayload)) {
	// Create a worker pool with limited concurrency
	numWorkers := hc.concurrency
	sem := make(chan struct{}, numWorkers)

	// Create a wait group to track active workers
	var wg sync.WaitGroup

	// Process messages until context is cancelled
	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, stopping message consumption")
			wg.Wait() // Wait for in-progress workers to complete
			return
		default:
			// Add a worker to the semaphore
			select {
			case sem <- struct{}{}:
				// Only proceed if we can acquire the semaphore
				wg.Add(1)

				// Process message in a goroutine
				go func() {
					defer wg.Done()
					defer func() { <-sem }() // Release the semaphore when done

					// Try to fetch a message, starting with highest priority
					var msg jetstream.Msg
					var priority Priority
					var foundMessage bool

					// Check each priority in order until we find a message
					for _, p := range priorities {
						consumer := hc.consumers[p]
						batch, err := consumer.FetchNoWait(1)
						if err != nil {
							slog.Error("Failed to fetch message", "error", err, "priority", p)
							continue
						}

						// Simplify - if we have a message, use it directly
						msgs := batch.Messages()
						if m, ok := <-msgs; ok {
							slog.Debug("Found message", "subject", m.Subject, "priority", p)
							msg = m
							priority = p
							foundMessage = true
							break
						}
					}

					// If no message was found in any queue, just return and try again
					if !foundMessage {
						slog.Debug("No message found in any queue, sleeping")
						// Sleep briefly to avoid CPU spinning when queues are empty
						time.Sleep(1 * time.Second)
						return
					}
					slog.Debug("Found message", "subject", msg.Subject, "priority", priority)

					// Get the UUID from the ticket
					msg_id, err := uuid.FromBytes(msg.Data())
					if err != nil {
						slog.Error("Failed to parse message ID", "error", err, "data", string(msg.Data()))
						if err := msg.Nak(); err != nil {
							slog.Error("Failed to NAK message after parse error", "error", err, "subject", msg.Subject)
						}
						return
					}

					entry, err := hc.kv.Get(ctx, msg_id.String())
					if err != nil {
						slog.Error("Failed to get message from KeyValue store", "error", err, "id", msg_id.String())
						if err := msg.Nak(); err != nil {
							slog.Error("Failed to NAK message after KV store error", "error", err, "id", msg_id.String())
						}
						return
					}
					// Process the message
					var job payload.QueuePayload
					if err := json.Unmarshal(entry.Value(), &job); err != nil {
						slog.Error("Failed to unmarshal message payload", "error", err, "data", string(msg.Data()))
						if err := msg.Nak(); err != nil {
							slog.Error("Failed to NAK message after unmarshal error", "error", err, "data", string(msg.Data()))
						}
						return
					}

					slog.Info("Processing job", "id", job.ID.String(), "priority", priority, "subject", msg.Subject, "worker", fmt.Sprintf("%d/%d", len(sem), numWorkers))

					// Execute the processing function
					processing(job)

					slog.Info("Finished processing job", "id", job.ID.String(), "priority", priority, "subject", msg.Subject)
					// Acknowledge after processing
					if err := msg.Ack(); err != nil {
						slog.Error("Failed to acknowledge message", "error", err)
					}
				}()
			case <-ctx.Done():
				// Context was cancelled while waiting for a worker slot
				slog.Info("Context cancelled while waiting for worker")
				wg.Wait()
				return
			}
		}
	}
}
