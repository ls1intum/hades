package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	hades "github.com/ls1intum/hades/shared"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var _ hades.JobConsumer = (*HadesNATSConsumer)(nil)

// HadesNATSConsumer manages job consumption from the NATS queue system with priority handling.
type HadesNATSConsumer struct {
	natsConnection *nats.Conn
	concurrency    uint
	consumers      map[hades.Priority]jetstream.Consumer
	kv             jetstream.KeyValue
}

// NewHadesConsumer creates a new job consumer with priority queue support.
// The concurrency parameter controls the maximum number of jobs processed simultaneously.
func NewHadesConsumer(nc *nats.Conn, concurrency uint) (*HadesNATSConsumer, error) {
	ctx := context.Background()
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Error("Failed to create JetStream context", "error", err)
		return nil, err
	}
	consumers := make(map[hades.Priority]jetstream.Consumer, len(hades.Priorities))

	// Create a consumer for each priority (one consumer is shared across all worker nodes)
	for _, priority := range hades.Priorities {
		consumerName := fmt.Sprintf("HADES_JOBS_%s", priority)
		cons, err := js.CreateOrUpdateConsumer(ctx, "HADES_JOBS", jetstream.ConsumerConfig{
			Name:          consumerName,
			Durable:       consumerName,
			AckPolicy:     jetstream.AckExplicitPolicy,
			FilterSubject: prioritySubject(priority),
		})
		if err != nil {
			slog.Error("Failed to create JetStream consumer", "error", err, "priority", priority)
			return nil, err
		}
		consumers[priority] = cons
		slog.Info("Created JetStream consumer", "consumer", consumerName, "priority", prioritySubject(priority))
	}

	slog.Info("Created JetStream consumer", "consumers", consumers)

	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "HADES_JOBS",
	})
	if err != nil {
		slog.Error("Failed to create JetStream KeyValue store", "error", err)
		return nil, err
	}
	return &HadesNATSConsumer{
		natsConnection: nc,
		consumers:      consumers,
		concurrency:    concurrency,
		kv:             kv,
	}, nil
}

// DequeueJob continuously processes jobs from the queue in priority order.
// It spawns worker goroutines up to the configured concurrency limit and processes
// jobs using the provided processing function. Workers fetch jobs on-demand to ensure
// fair distribution across multiple consumer instances.
func (hc *HadesNATSConsumer) DequeueJob(ctx context.Context, processing hades.PayloadHandler) {
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
func (hc *HadesNATSConsumer) workerLoop(ctx context.Context, workerID uint, processing hades.PayloadHandler) {
	// Exponential backoff for when no messages are available

	const resetBackoff = 10 * time.Millisecond
	const maxBackoff = 1 * time.Second
	backoff := resetBackoff

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
		backoff = resetBackoff

		// Parse and validate the message
		job, err := hc.parseMessage(ctx, msg)
		if err != nil {
			// Error already logged in parseMessage
			continue
		}

		if job.Metadata == nil {
			job.Metadata = map[string]string{}
		}
		job.Metadata["hades.tum.de/priority"] = strconv.Itoa(hades.PriorityToInt(priority))
		job.Metadata["hades.tum.de/priorityName"] = string(priority)

		// Process the job immediately (we have capacity)
		hc.processJob(workerID, job, msg, priority, processing)
	}
}

// processJob handles individual job processing with error recovery
func (hc *HadesNATSConsumer) processJob(
	workerID uint,
	job payload.QueuePayload,
	msg jetstream.Msg,
	priority hades.Priority,
	processing hades.PayloadHandler,
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
func (hc *HadesNATSConsumer) parseMessage(ctx context.Context, msg jetstream.Msg) (payload.QueuePayload, error) {
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
func (hc *HadesNATSConsumer) fetchNextMessage(ctx context.Context) (jetstream.Msg, hades.Priority, bool) {
	for _, p := range hades.Priorities {
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
