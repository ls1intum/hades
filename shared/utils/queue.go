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

const (
	natsConnectTimeout = 10 * time.Second
	natsReconnectWait  = 5 * time.Second
)

var (
	// priorities defines the order in which job queues are checked (high to low)
	priorities = []Priority{HighPriority, MediumPriority, LowPriority}
)

// HadesProducer manages job publishing to the NATS queue system.
type HadesProducer struct {
	natsConnection *nats.Conn
	js             jetstream.JetStream
	kv             jetstream.KeyValue
}

// HadesConsumer manages job consumption from the NATS queue system with priority handling.
type HadesConsumer struct {
	natsConnection *nats.Conn
	concurrency    uint
	consumers      map[Priority]jetstream.Consumer
	kv             jetstream.KeyValue
}

// SetupNatsConnection creates a connection to the NATS server with the provided configuration.
// It configures timeouts, reconnection behavior, and optional authentication/TLS.
func SetupNatsConnection(config NatsConfig) (*nats.Conn, error) {
	opts := []nats.Option{
		nats.Name("HadesAPI"),
		nats.Timeout(natsConnectTimeout),
		nats.ReconnectWait(natsReconnectWait),
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

// NewHadesProducer creates a new job producer with JetStream support.
// It initializes the job stream and key-value store for job metadata.
func NewHadesProducer(nc *nats.Conn) (*HadesProducer, error) {
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}

	s, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       "HADES_JOBS",
		Subjects:   []string{fmt.Sprintf("%s.*", natsSubjectBase)},
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

// NewHadesConsumer creates a new job consumer with priority queue support.
// The concurrency parameter controls the maximum number of jobs processed simultaneously.
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

// EnqueueHighJob adds a job to the high-priority queue.
func (hp *HadesProducer) EnqueueHighJob(ctx context.Context, queuePayload payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayload, HighPriority)
}

// EnqueueMediumJob adds a job to the medium-priority queue.
func (hp *HadesProducer) EnqueueMediumJob(ctx context.Context, queuePayload payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayload, MediumPriority)
}

// EnqueueLowJob adds a job to the low-priority queue.
func (hp *HadesProducer) EnqueueLowJob(ctx context.Context, queuePayload payload.QueuePayload) error {
	return hp.EnqueueJobWithPriority(ctx, queuePayload, LowPriority)
}

// EnqueueJobWithPriority adds a job to the queue with the specified priority.
// The job payload is stored in the key-value store and a reference is published to the stream.
func (hp *HadesProducer) EnqueueJobWithPriority(ctx context.Context, queuePayload payload.QueuePayload, priority Priority) error {
	bytesPayload, err := json.Marshal(queuePayload)
	if err != nil {
		slog.Error("Failed to marshal payload", "error", err)
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}
	// Store full job payload in key-value store first
	_, err = hp.kv.Put(ctx, queuePayload.ID.String(), bytesPayload)
	if err != nil {
		return fmt.Errorf("failed to store job payload in KV store: %w", err)
	}
	// Publish job reference to the stream (use synchronous publish for guaranteed delivery)
	_, err = hp.js.Publish(ctx, PrioritySubject(priority), queuePayload.ID[:], jetstream.WithMsgID(queuePayload.ID.String()))
	if err != nil {
		return fmt.Errorf("failed to publish job to stream: %w", err)
	}
	return nil
}

// DequeueJob continuously processes jobs from the queue in priority order.
// It spawns worker goroutines up to the configured concurrency limit and processes
// jobs using the provided processing function. Processing continues until the context
// is cancelled.
func (hc *HadesConsumer) DequeueJob(ctx context.Context, processing func(payload payload.QueuePayload)) {
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

					// Get the UUID from the message data
					msgID, err := uuid.FromBytes(msg.Data())
					if err != nil {
						slog.Error("Failed to parse message ID", "error", err, "data", string(msg.Data()))

						if err := msg.Nak(); err != nil {
							slog.Error("Failed to NAK message after parse error", "error", err, "subject", msg.Subject)
						}
						return
					}

					entry, err := hc.kv.Get(ctx, msgID.String())
					if err != nil {
						slog.Error("Failed to get message from KeyValue store", "error", err, "id", msgID.String())

						if err := msg.Nak(); err != nil {
							slog.Error("Failed to NAK message after KV store error", "error", err, "id", msgID.String())
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
