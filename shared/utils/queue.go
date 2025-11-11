package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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
// jobs using the provided processing function. Workers fetch jobs on-demand to ensure
// fair distribution across multiple consumer instances.
func (hc *HadesConsumer) DequeueJob(ctx context.Context, processing func(payload payload.QueuePayload)) {
	var wg sync.WaitGroup

	// Create workers that fetch their own jobs (pull model instead of push)
	for i := uint(0); i < hc.concurrency; i++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Worker panic recovered", "workerID", workerID, "panic", r)
				}
			}()

			// Each worker continuously fetches and processes jobs
			hc.workerLoop(ctx, workerID, processing)
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()
	slog.Info("All workers finished, dequeue complete")
}

// workerLoop handles the fetch-process loop for a single worker
func (hc *HadesConsumer) workerLoop(ctx context.Context, workerID uint, processing func(payload payload.QueuePayload)) {
	// Exponential backoff for when no messages are available
	backoff := 10 * time.Millisecond
	const maxBackoff = 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, worker stopping", "workerID", workerID)
			return
		default:
		}

		// Only fetch when this worker is ready to process
		msg, priority, found := hc.fetchNextMessage(ctx)

		// If no message was found in any queue, sleep with backoff and retry
		if !found {
			select {
			case <-time.After(backoff):
				backoff = min(backoff*2, maxBackoff)
				continue
			case <-ctx.Done():
				slog.Info("Context cancelled during backoff", "workerID", workerID)
				return
			}
		}

		// Reset backoff on successful fetch
		backoff = 10 * time.Millisecond

		// Parse and validate the message
		job, err := hc.parseMessage(ctx, msg)
		if err != nil {
			// Error already logged in parseMessage
			continue
		}

		// Process the job immediately (we have capacity)
		hc.processJob(workerID, job, msg, priority, processing)
	}
}

// processJob handles individual job processing with error recovery
func (hc *HadesConsumer) processJob(
	workerID uint,
	job payload.QueuePayload,
	msg jetstream.Msg,
	priority Priority,
	processing func(payload payload.QueuePayload),
) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Job processing panic",
				"workerID", workerID,
				"jobID", job.ID.String(),
				"panic", r)
			// NAK on panic so job can be retried after a delay
			if err := msg.NakWithDelay(5 * time.Second); err != nil {
				slog.Error("Failed to NAK message after panic", "error", err)
			}
			return
		}
	}()

	slog.Info("Worker starting job",
		"workerID", workerID,
		"jobID", job.ID.String(),
		"priority", priority)

	processing(job)

	slog.Info("Worker finished job",
		"workerID", workerID,
		"jobID", job.ID.String(),
		"priority", priority)

	// Acknowledge after successful processing
	if err := msg.Ack(); err != nil {
		slog.Error("Failed to acknowledge message",
			"error", err,
			"jobID", job.ID.String())
	}
}

// parseMessage handles message parsing and validation
func (hc *HadesConsumer) parseMessage(ctx context.Context, msg jetstream.Msg) (payload.QueuePayload, error) {
	var job payload.QueuePayload

	// Get the UUID from the message data
	msgID, err := uuid.FromBytes(msg.Data())
	if err != nil {
		slog.Error("Failed to parse message ID", "error", err, "data", string(msg.Data()))
		// Terminate message on parse error (won't be retried)
		if termErr := msg.Term(); termErr != nil {
			slog.Error("Failed to terminate message after parse error", "error", termErr)
		}
		return job, fmt.Errorf("parse message ID: %w", err)
	}

	entry, err := hc.kv.Get(ctx, msgID.String())
	if err != nil {
		// Handle missing payload specifically
		if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrBucketNotFound) {
			slog.Error("Job payload missing from KeyValue store", "id", msgID.String(), "error", err)
			if termErr := msg.Term(); termErr != nil {
				slog.Error("Failed to terminate message after missing payload", "error", termErr, "id", msgID.String())
			}
			return job, fmt.Errorf("job payload missing: %w", err)
		}
		slog.Error("Failed to get message from KeyValue store", "error", err, "id", msgID.String())
		// NAK with delay for transient KV errors
		if nakErr := msg.NakWithDelay(5 * time.Second); nakErr != nil {
			slog.Error("Failed to NAK message after KV store error", "error", nakErr)
		}
		return job, fmt.Errorf("get from KV store: %w", err)
	}

	if err := json.Unmarshal(entry.Value(), &job); err != nil {
		slog.Error("Failed to unmarshal message payload", "error", err, "id", msgID.String())
		// Terminate on unmarshal error (data corruption, won't be retried)
		if termErr := msg.Term(); termErr != nil {
			slog.Error("Failed to terminate message after unmarshal error", "error", termErr)
		}
		return job, fmt.Errorf("unmarshal payload: %w", err)
	}

	return job, nil
}

// fetchNextMessage tries to fetch a message in strict priority order
func (hc *HadesConsumer) fetchNextMessage(ctx context.Context) (jetstream.Msg, Priority, bool) {
	for _, p := range priorities {
		consumer := hc.consumers[p]

		// Use FetchNoWait with proper cleanup
		batch, err := consumer.FetchNoWait(1)
		if err != nil {
			// Only log non-timeout errors
			if err != jetstream.ErrNoMessages {
				slog.Error("Failed to fetch message", "error", err, "priority", p)
			}
			continue
		}

		// Properly drain the messages channel
		msgs := batch.Messages()
		select {
		case msg, ok := <-msgs:
			if ok {
				// Drain any remaining messages (should be none with batch size 1)
				go func() {
					for range msgs {
						// Drain to prevent goroutine leak
					}
				}()
				slog.Debug("Found message", "subject", msg.Subject(), "priority", p)
				return msg, p, true
			}
		case <-ctx.Done():
			return nil, "", false
		}
	}

	return nil, "", false
}
