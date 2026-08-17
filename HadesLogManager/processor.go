package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/buildstatus"
)

const (
	httpClientTimeout = 10 * time.Second
	// defaultRetention is used when LOG_RETENTION is unset or non-positive. A
	// non-positive retention would otherwise make completed jobs eligible for
	// flushing immediately.
	defaultRetention = time.Hour
)

// NATSLogAggregator implements LogAggregator using in-memory storage for fast log retrieval.
// It provides thread-safe log aggregation with configurable batching, automatic log rotation,
// and memory management. Thread-safety is provided by sync.Map for all operations.
type NATSLogAggregator struct {
	hlc       *buildlogs.HadesLogConsumer
	logs      sync.Map // jobID (string) -> logsVersion
	completed sync.Map // jobID (string) -> time.Time (completion time)
	status    sync.Map // jobID (string) -> buildstatus.JobStatus
	cfg       AggregatorConfig
}

// wrapper stored in sync.Map: comparable (uint64 + pointer)
type logsVersion struct {
	ver uint64
	ptr *[]buildlogs.Log
}

// AggregatorConfig defines the configuration parameters for log aggregation behavior.
type AggregatorConfig struct {
	BatchSize   int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	Retention   time.Duration `env:"LOG_RETENTION" envDefault:"1h"`
	MaxJobLogs  int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
	APIendpoint string        `env:"ARTEMIS_ADAPTER_URL"`
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
func NewLogAggregator(ctx context.Context, hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) buildlogs.LogAggregator {
	// Normalize retention once so every downstream use (cleanup cadence and the
	// expiry comparison) works from a single, guaranteed-positive value.
	if config.Retention <= 0 {
		slog.Warn("Non-positive LOG_RETENTION; falling back to default",
			"configured", config.Retention, "default", defaultRetention)
		config.Retention = defaultRetention
	}

	la := &NATSLogAggregator{
		hlc: hlc,
		cfg: config,
	}

	// Start background cleanup goroutine
	go la.cleanupLoop(ctx)

	return la
}

// cleanupLoop runs periodic cleanup of completed jobs. It ticks at the
// configured retention interval (floored at one minute) so that completed jobs
// are flushed within roughly one retention period of expiring.
func (la *NATSLogAggregator) cleanupLoop(ctx context.Context) {
	interval := la.cfg.Retention
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
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

// AddLog merges an incoming log into the aggregator for the specified job.
//
// Logs are coalesced by ContainerID so that the stored slice holds exactly one
// buildlogs.Log per container, in first-seen order. Under live streaming a
// container emits many small Logs over its lifetime; appending each as a
// separate element would break the positional logs[index] == step contract that
// the Artemis adapter relies on. A zero-entry Log still registers the
// container's slot, so a step producing no output keeps its position.
//
// This method is thread-safe using sync.Map with compare-and-swap operations.
// It preserves copy-on-write semantics: the entries of the merged container are
// deep-copied so a concurrent GetJobLogs/SendJobLogs reader holding the previous
// slice never observes a mutated backing array.
//
// Parameters:
//   - log: The buildlogs.Log entry to add, must contain a valid JobID
func (la *NATSLogAggregator) AddLog(log buildlogs.Log) {
	if log.JobID == "" {
		slog.Warn("Attempted to add log with empty job ID")
		return
	}
	if log.ContainerID == "" {
		slog.Warn("Attempted to add log with empty container ID", "job_id", log.JobID)
		return
	}

	slog.Debug("Adding log to aggregator", "job_id", log.JobID, "container_id", log.ContainerID, "entries", len(log.Logs))

	// Use LoadOrStore and CompareAndSwap for thread-safe updates
	for {
		value, _ := la.logs.LoadOrStore(log.JobID, logsVersion{ver: 0, ptr: &[]buildlogs.Log{}})
		old := value.(logsVersion)
		existing := *old.ptr

		// Shallow-copy the outer slice. Elements we do not touch stay shared with
		// readers (their entries are never mutated in place); only the merged
		// container element is replaced with a freshly allocated entries slice.
		newLogs := make([]buildlogs.Log, len(existing), len(existing)+1)
		copy(newLogs, existing)

		idx := -1
		for i := range newLogs {
			if newLogs[i].ContainerID == log.ContainerID {
				idx = i
				break
			}
		}

		if idx == -1 {
			// First time we see this container: register its slot with a copy of
			// the incoming entries so we never alias the caller's slice.
			merged := append([]buildlogs.LogEntry(nil), log.Logs...)
			merged = la.trimContainerEntries(log.JobID, log.ContainerID, merged)
			newLogs = append(newLogs, buildlogs.Log{JobID: log.JobID, ContainerID: log.ContainerID, Logs: merged})
		} else {
			// Merge into the existing container. Deep-copy the entries into a new
			// backing array so readers of the old slice are unaffected.
			prev := newLogs[idx].Logs
			merged := make([]buildlogs.LogEntry, 0, len(prev)+len(log.Logs))
			merged = append(merged, prev...)
			merged = append(merged, log.Logs...)
			merged = la.trimContainerEntries(log.JobID, log.ContainerID, merged)
			newLogs[idx] = buildlogs.Log{JobID: log.JobID, ContainerID: log.ContainerID, Logs: merged}
		}

		newVal := logsVersion{ver: old.ver + 1, ptr: &newLogs}

		// Atomically swap if the value hasn't changed
		if la.logs.CompareAndSwap(log.JobID, old, newVal) {
			slog.Debug("Merged log into aggregator", "job_id", log.JobID, "container_id", log.ContainerID, "containers", len(newLogs))
			break
		}
		// If swap failed, another goroutine modified the value - retry
	}
}

// trimContainerEntries caps a single container's entries at MaxJobLogs, dropping
// the oldest entries of that container. It never drops a whole container element,
// which would shift step indices and break the Artemis logs[index] contract.
func (la *NATSLogAggregator) trimContainerEntries(jobID, containerID string, entries []buildlogs.LogEntry) []buildlogs.LogEntry {
	if la.cfg.MaxJobLogs > 0 && len(entries) > la.cfg.MaxJobLogs {
		trimStart := len(entries) - la.cfg.MaxJobLogs
		slog.Debug("Trimmed old container entries", "job_id", jobID, "container_id", containerID, "trimmed_count", trimStart)
		entries = entries[trimStart:]
	}
	return entries
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
		slog.Info("Flushed job logs", "job_id", jobID, "batch_count", len(logs))
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
	slog.Info("Marked job as completed", "job_id", jobID, "retention", la.cfg.Retention)
}

// cleanupCompletedJobs removes logs for jobs that have exceeded the retention period.
func (la *NATSLogAggregator) cleanupCompletedJobs() {
	now := time.Now()
	cleanedCount := 0

	la.completed.Range(func(key, value any) bool {
		jobID := key.(string)
		completedAt := value.(time.Time)

		if now.Sub(completedAt) >= la.cfg.Retention {
			slog.Debug("Retention expired, flushing job", "job_id", jobID)

			if err := la.FlushJob(jobID); err != nil {
				slog.Error("Failed to flush job during cleanup", "job_id", jobID, "error", err)
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

// SendJobLogs retrieves all stored logs for jobID, marshals them to JSON, and
// sends them via an HTTP POST to the configured APIendpoint. The request is
// bound to ctx so it is cancelled on shutdown. It returns an error if
// marshalling, request creation, the HTTP call, or a non-2xx response occurs.
//
// If no APIendpoint is configured, the call is a no-op.
func (la *NATSLogAggregator) SendJobLogs(ctx context.Context, jobID string) error {
	if la.cfg.APIendpoint == "" {
		slog.Debug("No log adapter endpoint configured, skipping log forwarding", "job_id", jobID)
		return nil
	}

	logs := la.GetJobLogs(jobID)

	jsonData, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("marshaling logs to JSON: %w", err)
	}
	slog.Debug("Marshaled logs to JSON", "job_id", jobID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, la.cfg.APIendpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("log adapter returned non-success status %s for job %s", resp.Status, jobID)
	}

	slog.Info("Sent job logs to adapter", "job_id", jobID, "status", resp.Status)
	return nil
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
func (la *NATSLogAggregator) GetJobLogs(jobID string) []buildlogs.Log {
	value, exists := la.logs.Load(jobID)
	if !exists {
		return []buildlogs.Log{}
	}

	v := value.(logsVersion)
	logs := *v.ptr
	return logs
}

// GetAllJobs returns a slice containing all job IDs that are running or completed
//
// This method is thread-safe using sync.Map.Range.
//
// Returns:
//   - []string: A slice of all job IDs currently stored in the aggregator
func (la *NATSLogAggregator) GetAllJobs() []string {
	jobs := make([]string, 0)

	la.status.Range(func(key, value any) bool {
		jobs = append(jobs, key.(string))
		return true // Continue iteration
	})

	return jobs
}

// UpdateJobStatus stores or overwrites the build status for jobID.
// This method is thread-safe via sync.Map.Store.
func (la *NATSLogAggregator) UpdateJobStatus(jobID string, status buildstatus.JobStatus) {
	la.status.Store(jobID, status)
}

// GetJobStatus returns the string representation of the current build status for jobID.
// It returns an error if no status has been recorded for the given job.
func (la *NATSLogAggregator) GetJobStatus(jobID string) (buildstatus.JobStatus, error) {
	value, exists := la.status.Load(jobID)
	if !exists {
		return buildstatus.JobStatus(""), fmt.Errorf("job not found: %s", jobID)
	}
	return value.(buildstatus.JobStatus), nil
}
