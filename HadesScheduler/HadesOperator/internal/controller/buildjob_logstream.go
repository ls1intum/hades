package controller

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// logStreamPersistThrottle bounds how often a container's streaming progress
// (LogsStreamedUntil) is written back to the BuildJob status. Persisting on every
// flush would trigger a reconcile/etcd write storm, so progress is tracked in
// memory and only persisted once this much wall-clock time has advanced.
const logStreamPersistThrottle = 30 * time.Second

// logStreamKey uniquely identifies a container's live log stream.
func logStreamKey(namespace, jobID, container string) string {
	return namespace + "/" + jobID + "/" + container
}

// logStream is a single running (or just-finished) container log-follow goroutine.
type logStream struct {
	cancel context.CancelFunc
	done   chan struct{}
	jobID  string

	mu     sync.Mutex
	lastTS time.Time // in-memory streaming progress
	err    error     // set before done is closed
}

func (s *logStream) recordProgress(t time.Time) {
	s.mu.Lock()
	if t.After(s.lastTS) {
		s.lastTS = t
	}
	s.mu.Unlock()
}

func (s *logStream) progress() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastTS
}

// result reports whether the stream goroutine has returned and, if so, its error.
func (s *logStream) result() (finished bool, err error) {
	select {
	case <-s.done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return true, s.err
	default:
		return false, nil
	}
}

// logStreamRegistry tracks the live log-follow goroutines the operator runs, one
// per container, so they can be started idempotently and cancelled when a job
// completes or is deleted. Goroutines are rooted at context.Background so they
// outlive the per-reconcile context; their lifetime is bounded by cancel.
type logStreamRegistry struct {
	mu      sync.Mutex
	streams map[string]*logStream
}

// NewLogStreamRegistry creates an empty registry.
func NewLogStreamRegistry() *logStreamRegistry {
	return &logStreamRegistry{streams: make(map[string]*logStream)}
}

// ensureStarted launches a streaming goroutine for key if one is not already
// running. It is idempotent: subsequent calls for the same key are no-ops while
// the stream exists. run receives a cancellable context and a progress recorder.
// It returns true only when it actually started a new stream, so the caller can
// perform one-time, ordered side effects (e.g. registering the container's slot).
func (r *logStreamRegistry) ensureStarted(key, jobID string, run func(ctx context.Context, progress func(time.Time)) error) (started bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.streams == nil {
		r.streams = make(map[string]*logStream)
	}
	if _, ok := r.streams[key]; ok {
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &logStream{cancel: cancel, done: make(chan struct{}), jobID: jobID}
	r.streams[key] = s

	go func() {
		err := run(ctx, s.recordProgress)
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		close(s.done)
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("Container log stream ended with error", "key", key, "error", err)
		}
	}()
	return true
}

// get returns the tracked stream for key, or nil.
func (r *logStreamRegistry) get(key string) *logStream {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.streams[key]
}

// remove cancels and forgets the stream for key.
func (r *logStreamRegistry) remove(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.streams[key]; ok {
		s.cancel()
		delete(r.streams, key)
	}
}

// stopJob cancels and forgets every stream belonging to jobID.
func (r *logStreamRegistry) stopJob(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, s := range r.streams {
		if s.jobID == jobID {
			s.cancel()
			delete(r.streams, key)
		}
	}
}
