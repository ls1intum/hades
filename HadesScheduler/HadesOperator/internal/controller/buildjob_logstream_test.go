package controller

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestThrottledStreamOffset(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// No progress yet: keep whatever was persisted (nil).
	if got := throttledStreamOffset(nil, time.Time{}); got != nil {
		t.Fatalf("expected nil when no progress, got %v", got)
	}

	// First progress with no persisted value: persist it.
	got := throttledStreamOffset(nil, base)
	if got == nil || !got.Time.Equal(base) {
		t.Fatalf("expected first progress to be persisted, got %v", got)
	}

	// Progress advanced by less than the throttle: keep the persisted value.
	persisted := metav1.NewTime(base)
	if got := throttledStreamOffset(&persisted, base.Add(5*time.Second)); !got.Time.Equal(base) {
		t.Fatalf("expected persisted value kept under throttle, got %v", got.Time)
	}

	// Progress advanced by at least the throttle: persist the new value.
	adv := base.Add(logStreamPersistThrottle)
	if got := throttledStreamOffset(&persisted, adv); !got.Time.Equal(adv) {
		t.Fatalf("expected new value persisted past throttle, got %v", got.Time)
	}
}

func TestLogStreamRegistry_EnsureStartedIdempotentAndResult(t *testing.T) {
	reg := NewLogStreamRegistry()
	key := logStreamKey("ns", "job-1", "step-1")

	started := make(chan struct{})
	release := make(chan struct{})
	var runs int

	run := func(ctx context.Context, progress func(time.Time)) error {
		runs++
		close(started)
		progress(time.Unix(100, 0))
		<-release
		return nil
	}

	reg.ensureStarted(key, run)
	<-started
	// Second call must not start a second goroutine.
	reg.ensureStarted(key, func(context.Context, func(time.Time)) error {
		t.Error("run should not be called again while the stream exists")
		return nil
	})

	s := reg.get(key)
	if s == nil {
		t.Fatal("expected stream to be tracked")
	}
	if !s.progress().Equal(time.Unix(100, 0)) {
		t.Fatalf("expected progress recorded, got %v", s.progress())
	}
	if finished, _ := s.result(); finished {
		t.Fatal("stream should still be running")
	}

	close(release)
	// Wait for the goroutine to finish.
	deadline := time.After(2 * time.Second)
	for {
		if finished, err := s.result(); finished {
			if err != nil {
				t.Fatalf("unexpected stream error: %v", err)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("stream did not finish in time")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	if runs != 1 {
		t.Fatalf("expected run to execute exactly once, got %d", runs)
	}
}

func TestLogStreamRegistry_RemoveCancelsContext(t *testing.T) {
	reg := NewLogStreamRegistry()
	key := logStreamKey("ns", "job-1", "step-1")

	cancelled := make(chan struct{})
	reg.ensureStarted(key, func(ctx context.Context, _ func(time.Time)) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})

	reg.remove(key)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("remove did not cancel the stream context")
	}
	if reg.get(key) != nil {
		t.Fatal("expected stream to be forgotten after remove")
	}
}

func TestLogStreamRegistry_StopJobCancelsAllForJob(t *testing.T) {
	reg := NewLogStreamRegistry()

	block := func(ctx context.Context, _ func(time.Time)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	reg.ensureStarted(logStreamKey("ns", "job-1", "step-1"), block)
	reg.ensureStarted(logStreamKey("ns", "job-1", "step-2"), block)
	reg.ensureStarted(logStreamKey("ns", "job-2", "step-1"), block)
	// Same job name in a different namespace must not be affected (cluster-wide mode).
	reg.ensureStarted(logStreamKey("other-ns", "job-1", "step-1"), block)

	reg.stopJob("ns", "job-1")

	if reg.get(logStreamKey("ns", "job-1", "step-1")) != nil || reg.get(logStreamKey("ns", "job-1", "step-2")) != nil {
		t.Fatal("expected all job-1 streams removed")
	}
	if reg.get(logStreamKey("ns", "job-2", "step-1")) == nil {
		t.Fatal("expected job-2 stream to remain")
	}
	if reg.get(logStreamKey("other-ns", "job-1", "step-1")) == nil {
		t.Fatal("expected same-named job in another namespace to remain (no cross-namespace cancel)")
	}
}

// A stream that finishes with an error can be dropped and re-established, which is
// how the operator retries a follow that failed while its container is still
// running (via ensureStarted on the next reconcile).
func TestLogStreamRegistry_RestartAfterFailure(t *testing.T) {
	reg := NewLogStreamRegistry()
	key := logStreamKey("ns", "job-1", "step-1")

	var runs int32
	if !reg.ensureStarted(key, func(context.Context, func(time.Time)) error {
		atomic.AddInt32(&runs, 1)
		return errors.New("stream failed")
	}) {
		t.Fatal("first ensureStarted should start a stream")
	}

	// Wait for the first run to finish (with error).
	s := reg.get(key)
	deadline := time.After(2 * time.Second)
	for {
		if finished, _ := s.result(); finished {
			break
		}
		select {
		case <-deadline:
			t.Fatal("stream did not finish")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	// While the entry is still tracked, ensureStarted is a no-op (would leave a
	// failed stream dead) - the caller must remove it first.
	if reg.ensureStarted(key, func(context.Context, func(time.Time)) error { return nil }) {
		t.Fatal("ensureStarted should not restart while the finished entry is still tracked")
	}
	reg.remove(key)
	if !reg.ensureStarted(key, func(context.Context, func(time.Time)) error {
		atomic.AddInt32(&runs, 1)
		return nil
	}) {
		t.Fatal("ensureStarted should restart after the failed entry is removed")
	}

	// Wait for the restarted stream to run before asserting.
	s2 := reg.get(key)
	deadline = time.After(2 * time.Second)
	for {
		if finished, _ := s2.result(); finished {
			break
		}
		select {
		case <-deadline:
			t.Fatal("restarted stream did not finish")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	if got := atomic.LoadInt32(&runs); got != 2 {
		t.Fatalf("expected 2 runs (fail then restart), got %d", got)
	}
}

func TestLogStream_ResultReportsError(t *testing.T) {
	reg := NewLogStreamRegistry()
	key := logStreamKey("ns", "job-1", "step-1")
	wantErr := errors.New("boom")

	reg.ensureStarted(key, func(context.Context, func(time.Time)) error {
		return wantErr
	})

	s := reg.get(key)
	deadline := time.After(2 * time.Second)
	for {
		if finished, err := s.result(); finished {
			if !errors.Is(err, wantErr) {
				t.Fatalf("expected error %v, got %v", wantErr, err)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("stream did not finish")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}
