package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

// LogAggregator defines the interface for aggregating and managing job logs
type LogAggregator interface {
	AddLog(log buildlogs.Log)
	FlushJobLogs(jobID string) error
	GetJobLogs(jobID string) []buildlogs.LogEntry
	GetAllJobs() []string
	MarkJobCompleted(jobID string)
}

// NATSLogAggregator implements LogAggregator using in-memory storage for fast log retrieval.
// It provides thread-safe log aggregation with configurable batching, automatic log rotation,
// and memory management.
type NATSLogAggregator struct {
	hlc       *buildlogs.HadesLogConsumer
	logs      sync.Map // jobID (string) -> []buildlogs.Log
	completed sync.Map // jobID (string) -> time.Time (completion time)
	config    AggregatorConfig
	mu        sync.RWMutex // Protects compound operations
}

// AggregatorConfig defines the configuration parameters for log aggregation behavior.
type AggregatorConfig struct {
	BatchSize  int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	Retention  time.Duration `env:"LOG_RETENTION" envDefault:"1h"`
	MaxJobLogs int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
}

// NewLogAggregator creates a new NATS-based LogAggregator instance with the specified configuration.
// It starts a background goroutine for periodic cleanup of completed jobs.
//
// Parameters:
//   - ctx: Context for controlling the lifecycle of background goroutines
//   - hlc: HadesLogConsumer instance (currently unused but kept for future extensibility)
//   - config: AggregatorConfig containing batching and limit settings
//
// Returns:
//   - LogAggregator: A new instance ready to aggregate logs
func NewLogAggregator(ctx context.Context, hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) LogAggregator {
	la := &NATSLogAggregator{
		hlc:    hlc,
		config: config,
	}

	// Start background cleanup goroutine
	go la.cleanupLoop(ctx)

	return la
}

// cleanupLoop runs periodic cleanup of completed jobs
func (la *NATSLogAggregator) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping log aggregator cleanup goroutine")
			return
		case <-ticker.C:
			la.cleanupCompletedJobs()
		}
	}
}

// AddLog adds a new log entry to the aggregator for the specified job.
// It automatically creates a new log slice for new job IDs and trims old logs
// if the maximum log count per job is exceeded.
//
// This method is thread-safe using sync.Map with compare-and-swap operations.
//
// Parameters:
//   - log: The buildlogs.Log entry to add, must contain a valid JobID
func (la *NATSLogAggregator) AddLog(log buildlogs.Log) {
	if log.JobID == "" {
		slog.Warn("Attempted to add log with empty job ID")
		return
	}

	slog.Debug("Adding log to aggregator",
		"job_id", log.JobID,
		"entries", len(log.Logs))

	// Use LoadOrStore and CompareAndSwap for thread-safe updates
	for {
		value, _ := la.logs.LoadOrStore(log.JobID, []buildlogs.Log{})
		existingLogs := value.([]buildlogs.Log)

		// Create new slice with appended log
		newLogs := make([]buildlogs.Log, len(existingLogs), len(existingLogs)+1)
		copy(newLogs, existingLogs)
		newLogs = append(newLogs, log)

		// Trim if needed
		if len(newLogs) > la.config.MaxJobLogs {
			trimStart := len(newLogs) - la.config.MaxJobLogs
			newLogs = newLogs[trimStart:]
			slog.Debug("Trimmed old logs",
				"job_id", log.JobID,
				"trimmed_count", trimStart)
		}

		// Atomically swap if the value hasn't changed
		if la.logs.CompareAndSwap(log.JobID, existingLogs, newLogs) {
			slog.Debug("Added log to aggregator",
				"job_id", log.JobID,
				"total_batches", len(newLogs))
			break
		}
		// If swap failed, another goroutine modified the value - retry
	}
}

// FlushJobLogs removes logs for a completed job from memory.
// This is called after the retention period expires to free up memory.
//
// Parameters:
//   - jobID: The unique identifier for the job whose logs should be flushed
//
// Returns:
//   - error: Always nil in current implementation
func (la *NATSLogAggregator) FlushJobLogs(jobID string) error {
	value, exists := la.logs.LoadAndDelete(jobID)
	if !exists {
		slog.Debug("No logs to flush for job", "job_id", jobID)
		return nil
	}

	logs := value.([]buildlogs.Log)
	slog.Info("Flushed job logs",
		"job_id", jobID,
		"batch_count", len(logs))

	la.completed.Delete(jobID)
	return nil
}

// MarkJobCompleted marks a job as completed and schedules it for cleanup after retention period.
func (la *NATSLogAggregator) MarkJobCompleted(jobID string) {
	la.completed.Store(jobID, time.Now())
	slog.Info("Marked job as completed",
		"job_id", jobID,
		"retention", la.config.Retention)
}

// cleanupCompletedJobs removes logs for jobs that have exceeded the retention period.
func (la *NATSLogAggregator) cleanupCompletedJobs() {
	now := time.Now()
	cleanedCount := 0

	la.completed.Range(func(key, value any) bool {
		jobID := key.(string)
		completedAt := value.(time.Time)

		if now.Sub(completedAt) >= la.config.Retention {
			slog.Debug("Retention expired, flushing job logs", "job_id", jobID)

			if err := la.FlushJobLogs(jobID); err != nil {
				slog.Error("Failed to flush logs during cleanup",
					"job_id", jobID,
					"error", err)
			} else {
				cleanedCount++
			}
		}
		return true // Continue iteration
	})

	if cleanedCount > 0 {
		slog.Info("Completed log cleanup cycle",
			"cleaned_jobs", cleanedCount)
	}
}

// GetJobLogs retrieves all log entries for a specific job ID by flattening
// the batched logs into a single slice.
//
// This method is thread-safe using sync.Map.Load.
//
// Parameters:
//   - jobID: The unique identifier for the job whose logs to retrieve
//
// Returns:
//   - []buildlogs.LogEntry: All log entries for the specified job, or empty slice if not found
func (la *NATSLogAggregator) GetJobLogs(jobID string) []buildlogs.LogEntry {
	value, exists := la.logs.Load(jobID)
	if !exists {
		return []buildlogs.LogEntry{}
	}

	logs := value.([]buildlogs.Log)

	// Pre-calculate total size for efficient allocation
	totalEntries := 0
	for _, log := range logs {
		totalEntries += len(log.Logs)
	}

	allLogEntries := make([]buildlogs.LogEntry, 0, totalEntries)
	for _, log := range logs {
		allLogEntries = append(allLogEntries, log.Logs...)
	}

	return allLogEntries
}

// GetAllJobs returns a slice containing all job IDs that currently have logs
// stored in the aggregator.
//
// This method is thread-safe using sync.Map.Range.
//
// Returns:
//   - []string: A slice of all job IDs currently stored in the aggregator
func (la *NATSLogAggregator) GetAllJobs() []string {
	jobs := make([]string, 0)

	la.logs.Range(func(key, value any) bool {
		jobs = append(jobs, key.(string))
		return true // Continue iteration
	})

	return jobs
}
