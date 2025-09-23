package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

// LogAggregator collects and batches log entries of multiple jobs, providing
// in-memory storage with configurable batching and flushing behavior. It maintains
// logs per job ID and automatically trims old logs to prevent memory overflow.
type LogAggregator struct {
	hlc    *buildlogs.HadesLogConsumer
	logs   map[string][]buildlogs.Log // jobID -> logs
	mutex  sync.RWMutex
	config AggregatorConfig
}

// AggregatorConfig defines the configuration parameters for log aggregation behavior.
// It controls batching size, flush intervals, and memory limits per job.
type AggregatorConfig struct {
	BatchSize     int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	FlushInterval time.Duration `env:"LOG_FLUSH_INTERVAL" envDefault:"30s"`
	MaxJobLogs    int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
}

// NewLogAggregator creates a new LogAggregator instance with the specified configuration.
// It initializes the internal logs map and sets up the aggregator ready for use.
//
// Parameters:
//   - hlc: HadesLogConsumer instance for log operations
//   - config: AggregatorConfig containing batching and limit settings
//
// Returns:
//   - *LogAggregator: A new instance ready to aggregate logs
func NewLogAggregator(hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) *LogAggregator {
	return &LogAggregator{
		hlc:    hlc,
		logs:   make(map[string][]buildlogs.Log),
		config: config,
	}
}

// StartAggregating begins the log aggregation process with periodic flushing.
// It runs a background goroutine that flushes logs at regular intervals based
// on the configured FlushInterval. The method blocks until the context is canceled.
//
// Parameters:
//   - ctx: Context for controlling the aggregation lifecycle
//
// Returns:
//   - error: Context error when the aggregation is stopped
func (la *LogAggregator) StartAggregating(ctx context.Context) error {
	// Start periodic flush
	ticker := time.NewTicker(la.config.FlushInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				la.flushLogs()
			}
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

// addLog adds a new log entry to the aggregator for the specified job.
// It automatically creates a new log slice for new job IDs and trims old logs
// if the maximum log count per job is exceeded. The trimming keeps only the
// most recent logs to prevent memory overflow.
//
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - log: The buildlogs.Log entry to add to the aggregator
func (la *LogAggregator) addLog(log buildlogs.Log) {
	la.mutex.Lock()
	jobID := log.JobID

	if _, exists := la.logs[jobID]; !exists {
		la.logs[jobID] = make([]buildlogs.Log, 0, la.config.BatchSize)
	}

	la.logs[jobID] = append(la.logs[jobID], log)

	// Trim if too many logs for this job
	if len(la.logs[jobID]) > la.config.MaxJobLogs {
		// Keep only the latest logs
		start := len(la.logs[jobID]) - la.config.MaxJobLogs
		la.logs[jobID] = la.logs[jobID][start:]
	}

	logCount := len(la.logs[jobID])
	la.mutex.Unlock()
	slog.Debug("Added log to aggregator", "job_id", jobID, "total_logs", logCount)
}

// flushLogs processes batches of logs that have reached the configured batch size.
// Currently, this method only logs the flush operation but can be extended to
// send logs to external systems, databases, or other storage mechanisms.
//
// The method is called periodically by the background goroutine in StartAggregating
// and can also be called manually for immediate flushing.
//
// This method is thread-safe and acquires a write lock during execution.
func (la *LogAggregator) flushLogs() {
	la.mutex.Lock()
	defer la.mutex.Unlock()

	for jobID, logs := range la.logs {
		if len(logs) >= la.config.BatchSize {
			slog.Info("Flushing logs batch", "job_id", jobID, "batch_size", len(logs))
			// Here you could send to another system, save to DB, etc.
			// For now, we'll keep them in memory for the API
		}
	}
}

// API methods

// GetJobLogs retrieves all log entries for a specific job ID by flattening
// the batched logs into a single slice. Returns an empty slice if the job ID
// is not found in the aggregator.
//
// This method is thread-safe and uses a read lock to prevent data races.
//
// Parameters:
//   - jobID: The unique identifier for the job whose logs to retrieve
//
// Returns:
//   - []buildlogs.LogEntry: All log entries for the specified job, or empty slice if not found
func (la *LogAggregator) GetJobLogs(jobID string) []buildlogs.LogEntry {
	la.mutex.RLock()
	defer la.mutex.RUnlock()

	if logs, exists := la.logs[jobID]; exists {
		var allLogEntries []buildlogs.LogEntry
		for _, log := range logs {
			allLogEntries = append(allLogEntries, log.Logs...)
		}
		return allLogEntries
	}
	return []buildlogs.LogEntry{}
}

// GetAllJobs returns a slice containing all job IDs that currently have logs
// stored in the aggregator. This method is useful for discovering which jobs
// are being tracked and have associated log data.
//
// This method is thread-safe and uses a read lock to prevent data races.
//
// Returns:
//   - []string: A slice of all job IDs currently stored in the aggregator
func (la *LogAggregator) GetAllJobs() []string {
	la.mutex.RLock()
	defer la.mutex.RUnlock()

	jobs := make([]string, 0, len(la.logs))
	for jobID := range la.logs {
		jobs = append(jobs, jobID)
	}
	return jobs
}
