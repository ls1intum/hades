package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
)

func newTestAggregator(t *testing.T, maxJobLogs int) buildlogs.LogAggregator {
	t.Helper()
	return NewLogAggregator(context.Background(), nil, AggregatorConfig{
		BatchSize:  100,
		Retention:  time.Hour,
		MaxJobLogs: maxJobLogs,
	})
}

func logEntry(msg string) buildlogs.LogEntry {
	return buildlogs.LogEntry{Timestamp: time.Unix(0, 0), Message: msg, OutputStream: "stdout"}
}

// Streaming delivers many small Logs per container. They must coalesce into one
// element per container, in first-seen order, so the Artemis adapter's
// positional logs[1] == execute step contract holds.
func TestAddLog_CoalescesByContainerInOrder(t *testing.T) {
	agg := newTestAggregator(t, 1000)

	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-1", Logs: []buildlogs.LogEntry{logEntry("clone 1")}})
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-2", Logs: []buildlogs.LogEntry{logEntry("exec 1")}})
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-1", Logs: []buildlogs.LogEntry{logEntry("clone 2")}})
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-2", Logs: []buildlogs.LogEntry{logEntry("exec 2")}})
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-3", Logs: []buildlogs.LogEntry{logEntry("result 1")}})

	logs := agg.GetJobLogs("job-1")
	if len(logs) != 3 {
		t.Fatalf("expected one Log per container (3), got %d", len(logs))
	}

	want := []struct {
		container string
		messages  []string
	}{
		{"step-1", []string{"clone 1", "clone 2"}},
		{"step-2", []string{"exec 1", "exec 2"}},
		{"step-3", []string{"result 1"}},
	}
	for i, w := range want {
		if logs[i].ContainerID != w.container {
			t.Fatalf("logs[%d] container = %q, want %q", i, logs[i].ContainerID, w.container)
		}
		if len(logs[i].Logs) != len(w.messages) {
			t.Fatalf("logs[%d] entries = %d, want %d", i, len(logs[i].Logs), len(w.messages))
		}
		for j, m := range w.messages {
			if logs[i].Logs[j].Message != m {
				t.Fatalf("logs[%d].Logs[%d] = %q, want %q", i, j, logs[i].Logs[j].Message, m)
			}
		}
	}
}

// A step that produces no output must still occupy its slot so later steps do
// not shift into its index.
func TestAddLog_ZeroEntryLogRegistersSlot(t *testing.T) {
	agg := newTestAggregator(t, 1000)

	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-1", Logs: nil}) // clone: no output
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-2", Logs: []buildlogs.LogEntry{logEntry("exec")}})

	logs := agg.GetJobLogs("job-1")
	if len(logs) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(logs))
	}
	if logs[0].ContainerID != "step-1" || len(logs[0].Logs) != 0 {
		t.Fatalf("logs[0] = %+v, want empty step-1 slot", logs[0])
	}
	if logs[1].ContainerID != "step-2" {
		t.Fatalf("logs[1] container = %q, want step-2 (execute must stay at index 1)", logs[1].ContainerID)
	}
}

// Trimming caps entries within a container and must never drop a whole
// container element (which would shift step indices).
func TestAddLog_TrimsWithinContainerNotAcrossContainers(t *testing.T) {
	agg := newTestAggregator(t, 3)

	for i := 0; i < 10; i++ {
		agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-1", Logs: []buildlogs.LogEntry{logEntry(fmt.Sprintf("a%d", i))}})
	}
	agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-2", Logs: []buildlogs.LogEntry{logEntry("b0")}})

	logs := agg.GetJobLogs("job-1")
	if len(logs) != 2 {
		t.Fatalf("expected 2 containers preserved, got %d", len(logs))
	}
	if len(logs[0].Logs) != 3 {
		t.Fatalf("step-1 entries = %d, want capped at 3", len(logs[0].Logs))
	}
	// The tail (newest) entries are kept.
	if logs[0].Logs[0].Message != "a7" || logs[0].Logs[2].Message != "a9" {
		t.Fatalf("step-1 kept wrong entries: %q..%q, want a7..a9", logs[0].Logs[0].Message, logs[0].Logs[2].Message)
	}
	if logs[1].ContainerID != "step-2" {
		t.Fatalf("step-2 slot lost after trimming step-1")
	}
}

// The copy-on-write merge must not mutate a slice a concurrent reader holds.
// Run with -race to catch aliasing regressions.
func TestAddLog_ConcurrentReadersNoRace(t *testing.T) {
	agg := newTestAggregator(t, 100000)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				for _, l := range agg.GetJobLogs("job-1") {
					for _, e := range l.Logs {
						_ = e.Message
					}
				}
			}
		}
	}()

	for i := 0; i < 2000; i++ {
		agg.AddLog(buildlogs.Log{JobID: "job-1", ContainerID: "step-1", Logs: []buildlogs.LogEntry{logEntry(fmt.Sprintf("m%d", i))}})
	}
	close(stop)
	wg.Wait()
}
