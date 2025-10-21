package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

type LogManager interface {
	StartListening(ctx context.Context) error
}

// DynamicLogManager manages dynamic subscription to job logs based on job status changes.
// It automatically starts watching logs when a job begins executing and stops when the job
// finishes or fails. The manager maintains a map of active watchers to prevent duplicate
// subscriptions and ensure proper cleanup.
type DynamicLogManager struct {
	nc             *nats.Conn
	logConsumer    *logs.HadesLogConsumer
	logAggregator  LogAggregator
	activeWatchers map[string]context.CancelFunc // jobID -> cancel function
	mu             sync.RWMutex
}

// NewDynamicLogManager creates a new DynamicLogManager instance with the provided dependencies.
// It initializes the activeWatchers map and sets up all required components for managing
// dynamic log subscriptions.
//
// Parameters:
//   - nc: NATS connection for subscribing to job status events
//   - logConsumer: HadesLogConsumer for reading job logs
//   - aggregator: LogAggregator for storing and processing logs
//
// Returns:
//   - *DynamicLogManager: A new instance ready to start listening for job status changes
func NewDynamicLogManager(nc *nats.Conn, logConsumer *logs.HadesLogConsumer, aggregator LogAggregator) LogManager {
	return &DynamicLogManager{
		nc:             nc,
		logConsumer:    logConsumer,
		logAggregator:  aggregator,
		activeWatchers: make(map[string]context.CancelFunc),
	}
}

// StartListening begins listening for job status changes on NATS subjects and manages
// log watching accordingly. It subscribes to three status events:
//   - hades.status.running (logs.StatusRunning.Subject()): Starts log watching for the job
//   - hades.status.success/ hades.status.failed: Stops log watching for the job
//
// and adds the logs received to the log aggregator. The method expects job IDs to be sent
// as string data in NATS messages.
//
// Parameters:
//   - ctx: Context for managing the lifecycle of subscriptions
//
// Returns:
//   - error: Any error that occurred while setting up NATS subscriptions
func (dls *DynamicLogManager) StartListening(ctx context.Context) error {
	// Helper to extract jobID from message
	extractJobID := func(msg *nats.Msg) (string, bool) {
		if len(msg.Data) == 0 {
			slog.Error("Empty jobID received")
			return "", false
		}
		return string(msg.Data), true
	}

	// Track subscriptions for cleanup
	var subs []*nats.Subscription

	// Subscribe to executing status - start watching logs
	sub1, err := dls.nc.Subscribe(logs.StatusRunning.Subject(), func(msg *nats.Msg) {
		jobID, ok := extractJobID(msg)
		if !ok {
			return
		}
		slog.Info("Job started running", "job_id", jobID)
		dls.startWatchingJobLogs(ctx, jobID)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to running status: %w", err)
	}

	// Subscribe to completed status - stop watching logs
	sub2, err := dls.nc.Subscribe(logs.StatusSuccess.Subject(), func(msg *nats.Msg) {
		jobID, ok := extractJobID(msg)
		if !ok {
			return
		}
		slog.Info("Job success", "job_id", jobID)
		dls.stopWatchingJobLogs(jobID)
	})
	if err != nil {
		return fmt.Errorf("unable to subscribe to success status: %w", err)
	}

	// Subscribe to failed status - stop watching logs
	sub3, err := dls.nc.Subscribe(logs.StatusFailed.Subject(), func(msg *nats.Msg) {
		jobID, ok := extractJobID(msg)
		if !ok {
			return
		}
		slog.Info("Job failed", "job_id", jobID)
		dls.stopWatchingJobLogs(jobID)
	})
	if err != nil {
		return fmt.Errorf("unable to subscribe to failed status: %w", err)
	}

	subs = append(subs, sub1, sub2, sub3)

	// Clean up subscriptions when context is done
	go func() {
		<-ctx.Done()
		for _, sub := range subs {
			if err := sub.Drain(); err != nil {
				slog.Error("Failed to drain subscription", "error", err)
			}
		}
	}()

	return nil
}

// startWatchingJobLogs initiates log watching for a specific job according to jobID. If a
// watcher already exists, it cancels the existing one before starting a new one.
// The method runs the log watching in a separate goroutine to avoid blocking.
//
// The log watcher receives batched logs through a callback function and forwards them
// to the log aggregator. It automatically cleans up the watcher from the activeWatchers
// map when the goroutine exits.
//
// Parameters:
//   - ctx: Parent context for creating the job-specific context
//   - jobID: Unique identifier for the job to watch logs for
func (dls *DynamicLogManager) startWatchingJobLogs(ctx context.Context, jobID string) {
	dls.mu.Lock()

	// Cancel existing watcher if any
	if cancel, exists := dls.activeWatchers[jobID]; exists {
		cancel()
	}

	// Create new context for this job
	jobCtx, cancel := context.WithCancel(ctx)
	dls.activeWatchers[jobID] = cancel
	dls.mu.Unlock()

	// Start watching logs for this job
	go func() {
		defer func() {
			dls.mu.Lock()
			delete(dls.activeWatchers, jobID)
			dls.mu.Unlock()
		}()

		slog.Info("Starting to watch job logs", "job_id", jobID)
		err := dls.logConsumer.WatchJobLogs(jobCtx, jobID, func(batchedLog logs.Log) {
			// Store batched logs in aggregator
			dls.logAggregator.AddLog(batchedLog)

			slog.Info("Received batched job logs",
				"job_id", batchedLog.JobID,
				"container_id", batchedLog.ContainerID,
				"log_count", len(batchedLog.Logs))
		})

		if err != nil && err != context.Canceled {
			slog.Error("Error watching job logs", "job_id", jobID, "error", err)
		}
	}()
}

// stopWatchingJobLogs stops log watching for a specific job by canceling its context
// and removing it from the activeWatchers map. If no watcher exists for the given
// job ID, the method returns without error.
//
// This method is thread-safe and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - jobID: Unique identifier of the job to stop watching logs for
func (dls *DynamicLogManager) stopWatchingJobLogs(jobID string) {
	dls.mu.Lock()
	defer dls.mu.Unlock()

	cancel, exists := dls.activeWatchers[jobID]
	if exists {
		delete(dls.activeWatchers, jobID)
		slog.Info("Stopping log watch", "job_id", jobID)
		cancel()

		// if err := dls.logAggregator.FlushJobLogs(jobID); err != nil {
		// 	slog.Error("Failed to flush logs for completed job",
		// 		"job_id", jobID,
		// 		"error", err)
		// }
	}
}
