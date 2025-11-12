package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

var (
	// ErrEmptyJobID is returned when job ID is empty
	ErrEmptyJobID = errors.New("empty job ID")
)

// LogManager defines the interface for managing job log subscriptions
type LogManager interface {
	StartListening(ctx context.Context) error
}

// DynamicLogManager manages dynamic subscription to job logs based on job status changes.
// It automatically starts watching logs when a job begins executing and stops when the job
// finishes or fails. The manager maintains a map of active watchers to prevent duplicate
// subscriptions and ensure proper cleanup.
type DynamicLogManager struct {
	nc            *nats.Conn
	logConsumer   *logs.HadesLogConsumer
	logAggregator LogAggregator
	mu            sync.RWMutex
	watchers      map[string]watcherState // jobID -> watcher state
}

// watcherState holds the state for a single job watcher
type watcherState struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDynamicLogManager creates a new DynamicLogManager instance with the provided dependencies.
// It initializes internal state and sets up all required components for managing
// dynamic log subscriptions.
//
// Parameters:
//   - nc: NATS connection for subscribing to job status events
//   - logConsumer: HadesLogConsumer for reading job logs
//   - aggregator: LogAggregator for storing and processing logs
//
// Returns:
//   - LogManager: A new instance ready to start listening for job status changes
func NewDynamicLogManager(nc *nats.Conn, logConsumer *logs.HadesLogConsumer, aggregator LogAggregator) LogManager {
	return &DynamicLogManager{
		nc:            nc,
		logConsumer:   logConsumer,
		logAggregator: aggregator,
		watchers:      make(map[string]watcherState),
	}
}

// StartListening begins listening for job status changes on NATS subjects and manages
// log watching accordingly. It subscribes to three status events:
//   - hades.jobstatus.running: Starts log watching for the job
//   - hades.jobstatus.success/failed: Stops log watching for the job
//
// The method expects job IDs to be sent as string data in NATS messages.
//
// Parameters:
//   - ctx: Context for managing the lifecycle of subscriptions
//
// Returns:
//   - error: Any error that occurred while setting up NATS subscriptions
func (dlm *DynamicLogManager) StartListening(ctx context.Context) error {
	subs := make([]*nats.Subscription, 0, 3)

	// Subscribe to running status - start watching logs
	sub, err := dlm.subscribeToStatus(ctx, logs.StatusRunning, dlm.handleJobRunning)
	if err != nil {
		return err
	}
	subs = append(subs, sub)

	// Subscribe to completed status - stop watching logs
	sub, err = dlm.subscribeToStatus(ctx, logs.StatusSucceeded, dlm.handleJobCompleted)
	if err != nil {
		dlm.cleanupSubscriptions(subs)
		return err
	}
	subs = append(subs, sub)

	// Subscribe to failed status - stop watching logs
	sub, err = dlm.subscribeToStatus(ctx, logs.StatusFailed, dlm.handleJobCompleted)
	if err != nil {
		dlm.cleanupSubscriptions(subs)
		return err
	}
	subs = append(subs, sub)

	// Clean up subscriptions when context is done
	go func() {
		<-ctx.Done()
		slog.Info("Shutting down log manager subscriptions")
		dlm.cleanupSubscriptions(subs)
	}()

	return nil
}

// subscribeToStatus creates a subscription to a job status subject
func (dlm *DynamicLogManager) subscribeToStatus(ctx context.Context, status logs.JobStatus, handler func(context.Context, string)) (*nats.Subscription, error) {
	return dlm.nc.Subscribe(status.Subject(), func(msg *nats.Msg) {
		jobID, err := dlm.extractJobID(msg)
		if err != nil {
			slog.Warn("Invalid message received",
				"subject", msg.Subject,
				"error", err)
			return
		}
		handler(ctx, jobID)
	})
}

// extractJobID extracts and validates job ID from NATS message
func (dlm *DynamicLogManager) extractJobID(msg *nats.Msg) (string, error) {
	if len(msg.Data) == 0 {
		return "", ErrEmptyJobID
	}
	return string(msg.Data), nil
}

// handleJobRunning handles the job running status event
func (dlm *DynamicLogManager) handleJobRunning(ctx context.Context, jobID string) {
	slog.Info("Job started running", "job_id", jobID)
	dlm.startWatchingJobLogs(ctx, jobID)
}

// handleJobCompleted handles job completion (success or failure) events
func (dlm *DynamicLogManager) handleJobCompleted(ctx context.Context, jobID string) {
	slog.Info("Job completed", "job_id", jobID)
	dlm.stopWatchingJobLogs(jobID)
}

// cleanupSubscriptions drains all subscriptions
func (dlm *DynamicLogManager) cleanupSubscriptions(subs []*nats.Subscription) {
	for _, sub := range subs {
		if err := sub.Drain(); err != nil {
			slog.Warn("Failed to drain subscription", "error", err)
		}
	}
}

// startWatchingJobLogs initiates log watching for a specific job according to jobID.
// If a watcher already exists, it cancels the existing one before starting a new one.
// The method runs the log watching in a separate goroutine to avoid blocking.
//
// Parameters:
//   - ctx: Parent context for creating the job-specific context
//   - jobID: Unique identifier for the job to watch logs for
func (dlm *DynamicLogManager) startWatchingJobLogs(ctx context.Context, jobID string) {
	// Create new context for this job outside the lock
	jobCtx, cancel := context.WithCancel(ctx)

	// Minimize critical section - only lock for map operations
	dlm.mu.Lock()
	oldWatcher, exists := dlm.watchers[jobID]
	dlm.watchers[jobID] = watcherState{
		ctx:    jobCtx,
		cancel: cancel,
	}
	dlm.mu.Unlock()

	// Cancel old watcher outside the lock to avoid potential deadlock
	if exists {
		oldWatcher.cancel()
	}

	// Start watching logs for this job
	go func() {
		defer func() {
			// Use a more efficient cleanup check
			dlm.mu.Lock()
			if watcher, ok := dlm.watchers[jobID]; ok && watcher.ctx == jobCtx {
				delete(dlm.watchers, jobID)
			}
			dlm.mu.Unlock()
		}()

		slog.Info("Starting to watch job logs", "job_id", jobID)
		err := dlm.logConsumer.WatchJobLogs(jobCtx, jobID, func(batchedLog logs.Log) {
			dlm.logAggregator.AddLog(batchedLog)

			slog.Debug("Received batched job logs",
				"job_id", batchedLog.JobID,
				"container_id", batchedLog.ContainerID,
				"log_count", len(batchedLog.Logs))
		})

		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Error watching job logs",
				"job_id", jobID,
				"error", err)
		}
	}()
}

// stopWatchingJobLogs stops log watching for a specific job by canceling its context
// and removing it from the watchers map. If no watcher exists for the given
// job ID, the method returns without error.
//
// This method is thread-safe and can be called concurrently from multiple goroutines.
//
// Parameters:
//   - jobID: Unique identifier of the job to stop watching logs for
func (dlm *DynamicLogManager) stopWatchingJobLogs(jobID string) {
	dlm.mu.Lock()
	defer dlm.mu.Unlock()

	if watcher, exists := dlm.watchers[jobID]; exists {
		delete(dlm.watchers, jobID)
		slog.Info("Stopping log watch", "job_id", jobID)
		watcher.cancel()

		dlm.logAggregator.MarkJobCompleted(jobID)
	}
}
