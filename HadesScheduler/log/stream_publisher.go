package log

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	logs "github.com/ls1intum/hades/shared/buildlogs"
)

const (
	// defaultStreamFlushSize is the number of buffered entries that triggers an
	// immediate publish while following a container's logs.
	defaultStreamFlushSize = 50
	// defaultStreamFlushInterval bounds how long buffered entries wait before
	// being published, so live logs surface within roughly this interval even
	// when a container emits slowly.
	defaultStreamFlushInterval = 1 * time.Second
	// finalFlushTimeout bounds the detached publish of the last buffered entries
	// after the follow stream closes, so a slow NATS publish cannot hang the caller.
	finalFlushTimeout = 5 * time.Second
)

// StreamSource is a single readable log stream (e.g. stdout or stderr) tagged
// with the OutputStream it represents.
type StreamSource struct {
	Reader     io.Reader
	StreamType string
}

// StreamOptions configures optional behavior of StreamContainerLogs.
type StreamOptions struct {
	// Progress, if set, is called after each successful publish with the
	// timestamp of the last entry in the flushed batch. Callers use it to track
	// streaming progress (e.g. for a restart-safe follow offset). It is called
	// from the publish path and must be cheap and non-blocking.
	Progress func(time.Time)

	// RegisterSlot, when true, publishes a zero-entry Log up front so a container
	// producing no output still keeps its position in the aggregated per-step
	// ordering. Callers that run containers sequentially (the Docker executor) set
	// this; the operator registers slots itself, in deterministic order, because
	// it may start several containers' streams concurrently.
	RegisterSlot bool
}

// StreamContainerLogs follows one or more log streams of a single container,
// parses each line, and publishes the entries incrementally as small
// buildlogs.Log batches (one container per message). It is the shared streaming
// core used by both the Docker executor and the Kubernetes operator.
//
// It first publishes a zero-entry Log to register the container's slot, so a
// step that produces no output keeps its position in the aggregated per-step
// ordering the Artemis adapter relies on. Entries are flushed on
// defaultStreamFlushSize lines or defaultStreamFlushInterval, whichever comes
// first, plus a mandatory final flush once every source reaches EOF (the follow
// stream closes when the container stops).
//
// The function blocks until all sources are drained. Steady-state and final
// flushes preserve per-container order because a single mutex serializes the
// buffer even though the sources are scanned concurrently; LogEntry.Timestamp
// disambiguates stdout/stderr interleave.
func StreamContainerLogs(ctx context.Context, pub logs.LogPublisher, jobID, containerID string, opts StreamOptions, sources ...StreamSource) error {
	// Register the container's slot up front so a zero-output step is not skipped.
	if opts.RegisterSlot {
		if err := pub.PublishJobLog(ctx, logs.Log{JobID: jobID, ContainerID: containerID}); err != nil {
			slog.Warn("Failed to register container log slot", "job_id", jobID, "container_id", containerID, "error", err)
		}
	}

	var mu sync.Mutex
	var buf []logs.LogEntry

	flush := func(pubCtx context.Context) {
		mu.Lock()
		if len(buf) == 0 {
			mu.Unlock()
			return
		}
		batch := buf
		buf = nil
		mu.Unlock()

		if err := pub.PublishJobLog(pubCtx, logs.Log{JobID: jobID, ContainerID: containerID, Logs: batch}); err != nil {
			slog.Error("Failed to publish streamed logs", "job_id", jobID, "container_id", containerID, "entries", len(batch), "error", err)
			return
		}
		if opts.Progress != nil {
			opts.Progress(batch[len(batch)-1].Timestamp)
		}
	}

	add := func(e logs.LogEntry) {
		mu.Lock()
		buf = append(buf, e)
		n := len(buf)
		mu.Unlock()
		if n >= defaultStreamFlushSize {
			flush(ctx)
		}
	}

	// Periodic flush loop.
	ticker := time.NewTicker(defaultStreamFlushInterval)
	defer ticker.Stop()
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				flush(ctx)
			}
		}
	}()

	// Scan every source concurrently; a single mutex serializes publishing.
	var wg sync.WaitGroup
	var scanErr error
	var errMu sync.Mutex
	for _, src := range sources {
		wg.Add(1)
		go func(src StreamSource) {
			defer wg.Done()
			if err := ScanStream(src.Reader, src.StreamType, add); err != nil {
				errMu.Lock()
				if scanErr == nil {
					scanErr = err
				}
				errMu.Unlock()
			}
		}(src)
	}
	wg.Wait()
	close(stop)

	// Final flush on a detached context so the tail is published even if ctx was
	// cancelled (e.g. the job completed while the last lines were buffered).
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finalFlushTimeout)
	defer cancel()
	flush(finalCtx)

	return scanErr
}
