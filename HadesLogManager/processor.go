package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

type LogAggregator interface {
	AddLog(log buildlogs.Log)
	FlushJobLogs(jobID string) error
	GetJobLogs(jobID string) []buildlogs.LogEntry
	GetAllJobs() []string
	MarkJobCompleted(jobID string)
}

// NATSLogAggregator implements LogAggregator using NATS JetStream for log consumption
// and in-memory storage for fast log retrieval. It provides thread-safe log aggregation
// with configurable batching, automatic log rotation, and memory management.
//
// The aggregator maintains logs per job ID and automatically trims old logs to prevent
// memory overflow.
type NATSLogAggregator struct {
	hlc       *buildlogs.HadesLogConsumer
	logs      sync.Map // jobID (string) -> []buildlogs.Log
	completed sync.Map // jobID (string) -> time.Time (completion time)
	config    AggregatorConfig
}

// AggregatorConfig defines the configuration parameters for log aggregation behavior.
// It controls batching size, retention time, and memory limits per job.
type AggregatorConfig struct {
	BatchSize  int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	Retention  time.Duration `env:"LOG_RETENTION" envDefault:"1hr"`
	MaxJobLogs int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
}

// NewLogAggregator creates a new NATS-based LogAggregator instance with the specified configuration.
// It initializes the internal log storage and sets up the aggregator ready for use.
//
// Parameters:
//   - hlc: HadesLogConsumer instance for receiving logs from NATS JetStream
//   - config: AggregatorConfig containing batching and limit settings
//
// Returns:
//   - LogAggregator: A new instance ready to aggregate logs
func NewLogAggregator(ctx context.Context, hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) LogAggregator {
	la := &NATSLogAggregator{
		hlc:       hlc,
		logs:      sync.Map{},
		completed: sync.Map{},
		config:    config,
	}

	// start background cleanup goroutine
	go func() {
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
	}()

	return la
}

// addLog adds a new log entry to the aggregator for the specified job.
// It automatically creates a new log slice for new job IDs and trims old logs
// if the maximum log count per job is exceeded. The trimming keeps only the
// most recent logs to prevent memory overflow.
//
// This method is thread-safe using sync.Map operations and can be called concurrently
// from multiple goroutines without additional synchronization.
//
// Parameters:
//   - log: The buildlogs.Log entry to add to the aggregator, must contain a valid JobID
func (la *NATSLogAggregator) AddLog(log buildlogs.Log) {
	jobID := log.JobID
	slog.Debug("Start adding log to aggregator", "job_id", jobID)

	for {
		// Load current logs
		value, _ := la.logs.LoadOrStore(jobID, []buildlogs.Log{})
		existingLogs := value.([]buildlogs.Log)

		// Create a new slice (copy + append)
		newLogs := make([]buildlogs.Log, len(existingLogs), len(existingLogs)+1)
		copy(newLogs, existingLogs)
		newLogs = append(newLogs, log)

		// Trim if needed
		if len(newLogs) > la.config.MaxJobLogs {
			start := len(newLogs) - la.config.MaxJobLogs
			newLogs = newLogs[start:]
		}

		// Replace logs if existing
		if la.logs.CompareAndSwap(jobID, existingLogs, newLogs) {
			slog.Debug("Added log to aggregator", "job_id", jobID, "total_logs", len(newLogs))
			break
		}
	}
}

// FlushJobLogs processes batches of logs where the job has been completed/ failed.
// Currently, this method only logs the flush operation. Will be extended to
// send logs to external systems (Adapter).
//
// Parameters:
//   - jobID: The unique identifier for the job whose logs should be flushed
//
// Returns:
//   - error: Any error encountered during the flush operation (currently always nil)
func (la *NATSLogAggregator) FlushJobLogs(jobID string) error {
	value, exists := la.logs.LoadAndDelete(jobID)
	if !exists {
		slog.Warn("No logs to flush for completed job", "job_id", jobID)
		return nil
	}

	logs := value.([]buildlogs.Log)
	slog.Info("Flushed job logs", "job_id", jobID, "log_count", len(logs))
	la.completed.Delete(jobID)
	return nil
}

func (la *NATSLogAggregator) MarkJobCompleted(jobID string) {
	la.completed.Store(jobID, time.Now())
	slog.Info("Marked job as completed, will flush after retention", "job_id", jobID, "retention", la.config.Retention)
}

func (la *NATSLogAggregator) cleanupCompletedJobs() {
	now := time.Now()

	la.completed.Range(func(key, value any) bool {
		jobID := key.(string)
		completedAt := value.(time.Time)

		if now.Sub(completedAt) >= la.config.Retention {
			slog.Debug("Retention expired, flushing job logs", "job_id", jobID)

			if err := la.FlushJobLogs(jobID); err != nil {
				slog.Error("Failed to flush logs during cleanup", "job_id", jobID, "error", err)
			}
		}
		return true
	})
}

// API methods

// GetJobLogs retrieves all log entries for a specific job ID by flattening
// the batched logs into a single slice. Returns an empty slice if the job ID
// is not found in the aggregator.
//
// This method is thread-safe and uses sync.Map.Load for consistent reads.
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
// stored in the aggregator. This method is useful for discovering which jobs
// are being tracked and have associated log data.
//
// The returned slice contains job IDs in no particular order. For large numbers
// of jobs, consider pagination in the calling code.
//
// This method is thread-safe and uses sync.Map.Range for consistent iteration.
//
// Returns:
//   - []string: A slice of all job IDs currently stored in the aggregator
func (la *NATSLogAggregator) GetAllJobs() []string {
	jobs := make([]string, 0)
	la.logs.Range(func(key, value any) bool {
		jobs = append(jobs, key.(string))
		return true // continue iteration
	})
	return jobs
}
