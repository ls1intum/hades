package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

type LogAggregator struct {
	hlc    *buildlogs.HadesLogConsumer
	logs   map[string][]buildlogs.Log // jobID -> logs
	mutex  sync.RWMutex
	config AggregatorConfig
}

type AggregatorConfig struct {
	BatchSize     int           `env:"LOG_BATCH_SIZE" envDefault:"100"`
	FlushInterval time.Duration `env:"LOG_FLUSH_INTERVAL" envDefault:"30s"`
	MaxJobLogs    int           `env:"MAX_JOB_LOGS" envDefault:"1000"`
}

func NewLogAggregator(hlc *buildlogs.HadesLogConsumer, config AggregatorConfig) *LogAggregator {
	return &LogAggregator{
		hlc:    hlc,
		logs:   make(map[string][]buildlogs.Log),
		config: config,
	}
}

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

func (la *LogAggregator) addLog(log buildlogs.Log) {
	la.mutex.Lock()
	defer la.mutex.Unlock()

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

	slog.Debug("Added log to aggregator", "job_id", jobID, "total_logs", len(la.logs[jobID]))
}

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

func (la *LogAggregator) GetAllJobs() []string {
	la.mutex.RLock()
	defer la.mutex.RUnlock()

	jobs := make([]string, 0, len(la.logs))
	for jobID := range la.logs {
		jobs = append(jobs, jobID)
	}
	return jobs
}
