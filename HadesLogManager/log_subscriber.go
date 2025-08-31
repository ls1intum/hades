package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/nats-io/nats.go"
)

type DynamicLogManager struct {
	nc             *nats.Conn
	logConsumer    *logs.HadesLogConsumer
	logAggregator  *LogAggregator
	activeWatchers map[string]context.CancelFunc // jobID -> cancel function
	mu             sync.RWMutex
}

func NewDynamicLogManager(nc *nats.Conn, logConsumer *logs.HadesLogConsumer, aggregator *LogAggregator) *DynamicLogManager {
	return &DynamicLogManager{
		nc:             nc,
		logConsumer:    logConsumer,
		logAggregator:  aggregator,
		activeWatchers: make(map[string]context.CancelFunc),
	}
}

func (dls *DynamicLogManager) StartListening(ctx context.Context) error {
	// Subscribe to executing status - start watching logs
	_, err := dls.nc.Subscribe("hades.status.executing", func(msg *nats.Msg) {
		var jobID string
		if err := json.Unmarshal(msg.Data, &jobID); err != nil {
			slog.Error("Failed to unmarshal jobID from status: executing", "error", err)
			return
		}

		slog.Info("Job started executing", "job_id", jobID)
		dls.startWatchingJobLogs(ctx, jobID)
	})
	if err != nil {
		return fmt.Errorf("subscribing to executing status: %w", err)
	}

	// Subscribe to finished status - stop watching logs
	_, err = dls.nc.Subscribe("hades.status.finished", func(msg *nats.Msg) {
		var jobID string
		if err := json.Unmarshal(msg.Data, &jobID); err != nil {
			slog.Error("Failed to unmarshal jobID from status: finished", "error", err)
			return
		}

		slog.Info("Job finished", "job_id", jobID)
		dls.stopWatchingJobLogs(jobID)
	})
	if err != nil {
		return fmt.Errorf("subscribing to finished status: %w", err)
	}

	// Subscribe to failed status - stop watching logs
	_, err = dls.nc.Subscribe("hades.status.failed", func(msg *nats.Msg) {
		var jobID string
		if err := json.Unmarshal(msg.Data, &jobID); err != nil {
			slog.Error("Failed to unmarshal jobID from status: failed", "error", err)
			return
		}

		slog.Info("Job failed", "job_id", jobID)
		dls.stopWatchingJobLogs(jobID)
	})
	if err != nil {
		return fmt.Errorf("subscribing to failed status: %w", err)
	}

	return nil
}

func (dls *DynamicLogManager) startWatchingJobLogs(ctx context.Context, jobID string) {
	dls.mu.Lock()
	defer dls.mu.Unlock()

	// Cancel existing watcher if any
	if cancel, exists := dls.activeWatchers[jobID]; exists {
		cancel()
	}

	// Create new context for this job
	jobCtx, cancel := context.WithCancel(ctx)
	dls.activeWatchers[jobID] = cancel

	// Start watching logs for this job
	go func() {
		defer func() {
			dls.mu.Lock()
			delete(dls.activeWatchers, jobID)
			dls.mu.Unlock()
		}()

		slog.Info("Starting log watch", "job_id", jobID)
		err := dls.logConsumer.WatchJobLogs(jobCtx, jobID, func(batchedLog logs.Log) {
			// Store batched logs in aggregator
			dls.logAggregator.addLog(batchedLog)

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

func (dls *DynamicLogManager) stopWatchingJobLogs(jobID string) {
	dls.mu.Lock()
	defer dls.mu.Unlock()

	if cancel, exists := dls.activeWatchers[jobID]; exists {
		slog.Info("Stopping log watch", "job_id", jobID)
		cancel()
		delete(dls.activeWatchers, jobID)
	}
}
