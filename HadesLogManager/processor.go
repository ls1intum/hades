package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/buildstatus"
)

// LogAggregator defines the interface for aggregating and managing job logs
type LogAggregator interface {
	AddLog(log buildlogs.Log)
	FlushJob(jobID string) error
	GetJobLogs(jobID string) []buildlogs.Log
	GetAllJobs() []string
	MarkJobCompleted(jobID string)
	UpdateJobStatus(jobID string, status buildstatus.JobStatus)
	GetJobStatus(jobID string) (string, error)
}

// NATSLogAggregator implements LogAggregator using in-memory storage for fast log retrieval.
// It provides thread-safe log aggregation with configurable batching, automatic log rotation,
// and memory management. Thread-safety is provided by sync.Map for all operations.
type NATSLogAggregator struct {
	hlc       *buildlogs.HadesLogConsumer
	logs      sync.Map // jobID (string) -> logsVersion
	completed sync.Map // jobID (string) -> time.Time (completion time)
	status    sync.Map // jobID (string) -> buildstatus.JobStatus
	config    AggregatorConfig
}

// wrapper stored in sync.Map: comparable (uint64 + pointer)
type logsVersion struct {
	ver uint64
	ptr *[]buildlogs.Log
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
		value, _ := la.logs.LoadOrStore(log.JobID, logsVersion{ver: 0, ptr: &[]buildlogs.Log{}})
		old := value.(logsVersion)
		existing := *old.ptr

		// Create new slice with appended log
		newLogs := make([]buildlogs.Log, len(existing), len(existing)+1)
		copy(newLogs, existing)
		newLogs = append(newLogs, log)

		// Trim if needed
		if len(newLogs) > la.config.MaxJobLogs {
			trimStart := len(newLogs) - la.config.MaxJobLogs
			newLogs = newLogs[trimStart:]
			slog.Debug("Trimmed old logs",
				"job_id", log.JobID,
				"trimmed_count", trimStart)
		}

		newPtr := &newLogs
		newVal := logsVersion{ver: old.ver + 1, ptr: newPtr}

		// Atomically swap if the value hasn't changed
		if la.logs.CompareAndSwap(log.JobID, old, newVal) {
			slog.Debug("Added log to aggregator",
				"job_id", log.JobID,
				"total_batches", len(newLogs))
			break
		}
		// If swap failed, another goroutine modified the value - retry
	}
}

// FlushJob removes logs and status for a completed job from memory.
// This is called after the retention period expires to free up memory.
//
// Parameters:
//   - jobID: The unique identifier for the job whose data should be flushed
//
// Returns:
//   - error: Always nil in current implementation
func (la *NATSLogAggregator) FlushJob(jobID string) error {
	value, logsExists := la.logs.LoadAndDelete(jobID)
	if logsExists {
		v := value.(logsVersion)
		logs := *v.ptr
		slog.Info("Flushed job logs",
			"job_id", jobID,
			"batch_count", len(logs))
	} else {
		slog.Debug("No logs to flush for job", "job_id", jobID)
	}

	_, statusExists := la.status.LoadAndDelete(jobID)
	if !statusExists {
		slog.Debug("No status to flush for job", "job_id", jobID)
	}

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
			slog.Debug("Retention expired, flushing job", "job_id", jobID)

			if err := la.FlushJob(jobID); err != nil {
				slog.Error("Failed to flush job during cleanup",
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
//   - []buildlogs.Log: All logs of each container for the specified job, or empty slice if not found
//
// func (la *NATSLogAggregator) GetJobLogs(jobID string) []buildlogs.Log {
func (la *NATSLogAggregator) GetJobLogs(jobID string) []buildlogs.Log {

	value, exists := la.logs.Load(jobID)
	if !exists {
		return []buildlogs.Log{}
	}

	v := value.(logsVersion)
	logs := *v.ptr
	return logs
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

func (la *NATSLogAggregator) UpdateJobStatus(jobID string, status buildstatus.JobStatus) {
	la.status.Store(jobID, status)
}

func (la *NATSLogAggregator) GetJobStatus(jobID string) (string, error) {
	value, exists := la.status.Load(jobID)
	if !exists {
		slog.Error("Job not found", "job_id", jobID)
		return "", fmt.Errorf("job not found: %s", jobID)
	}
	return value.(buildstatus.JobStatus).String(), nil
}
