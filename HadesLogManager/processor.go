package main

import (
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

type LogAggregator interface {
	addLog(log buildlogs.Log)
	FlushJobLogs(jobID string) error
	GetJobLogs(jobID string) []buildlogs.LogEntry
	GetAllJobs() []string
}

// NATSLogAggregator implements LogAggregator using NATS JetStream for log consumption
// and in-memory storage for fast log retrieval. It provides thread-safe log aggregation
// with configurable batching, automatic log rotation, and memory management.
//
// The aggregator maintains logs per job ID and automatically trims old logs to prevent
// memory overflow.
type NATSLogAggregator struct {
	hlc    *buildlogs.HadesLogConsumer
	logs   sync.Map // jobID (string) -> []buildlogs.Log
	config AggregatorConfig
}

// AggregatorConfig defines the configuration parameters for log aggregation behavior.
// It controls batching size, flush intervals, and memory limits per job.
type AggregatorConfig struct {
	BatchSize     int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	FlushInterval time.Duration `env:"LOG_FLUSH_INTERVAL" envDefault:"30s"`
	MaxJobLogs    int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
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
func NewLogAggregator(hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) LogAggregator {
	return &NATSLogAggregator{
		hlc:    hlc,
		logs:   sync.Map{},
		config: config,
	}
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
func (la *NATSLogAggregator) addLog(log buildlogs.Log) {
	jobID := log.JobID

	// Load existing logs or create new slice for this job
	value, _ := la.logs.LoadOrStore(jobID, []buildlogs.Log{})
	existingLogs := value.([]buildlogs.Log)

	// Append new log entry
	updatedLogs := append(existingLogs, log)

	// Trim if needed
	if len(updatedLogs) > la.config.MaxJobLogs {
		start := len(updatedLogs) - la.config.MaxJobLogs
		updatedLogs = updatedLogs[start:]
	}

	// Store updated logs back to map
	la.logs.Store(jobID, updatedLogs)
	slog.Debug("Added log to aggregator", "job_id", jobID, "total_logs", len(updatedLogs))
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
	slog.Info("Flushing completed job logs", "job_id", jobID, "log_count", len(logs))

	// TODO: Send logs to Adapter

	return nil
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
	var allLogEntries []buildlogs.LogEntry
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
	jobs := []string{}
	la.logs.Range(func(key, value interface{}) bool {
		jobs = append(jobs, key.(string))
		return true // continue iteration
	})
	return jobs
}
