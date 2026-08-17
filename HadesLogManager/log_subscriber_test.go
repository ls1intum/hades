package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/ls1intum/hades/shared/buildlogs"
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const natsImage = "nats:2.11.4"

// startNATS spins up a JetStream-enabled NATS container and returns a connection to it.
func startNATS(t *testing.T) *nats.Conn {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        natsImage,
		ExposedPorts: []string{"4222/tcp", "8222/tcp"},
		Cmd:          []string{"-js", "-m", "8222"},
		WaitingFor:   wait.ForHTTP("/healthz").WithPort("8222/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start NATS container")
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			slog.Warn("Could not terminate NATS container", "error", err)
		}
	})

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err, "NATS endpoint")

	nc, err := hadesnats.SetupDefaultNatsConnection(hadesnats.ConnectionConfig{URL: "nats://" + endpoint})
	require.NoError(t, err, "connect to NATS")
	t.Cleanup(nc.Close)

	return nc
}

// TestStopWatchingDrainsAllContainerLogsBeforeCompletion reproduces the log-loss race:
// job logs travel over JetStream (durable, with latency) while the "succeeded" status
// travels over core NATS (instant). The operator publishes every container's log batch
// before announcing success, so by the time "succeeded" is handled the batches are all
// durably in the stream but not yet consumed. The previous behaviour hard-cancelled the
// watcher on "succeeded", abandoning the still-in-flight batches. This test publishes
// several containers' batches, then immediately drives the running -> succeeded path, and
// asserts the in-memory aggregation (which both the dashboard and the forward read) ends
// up with ALL containers' logs. It fails without the drain-before-forward fix and passes
// with it.
func TestStopWatchingDrainsAllContainerLogsBeforeCompletion(t *testing.T) {
	nc := startNATS(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	producer, err := buildlogs.NewHadesLogProducer(nc)
	require.NoError(t, err, "create log producer (also creates the stream)")

	consumer, err := buildlogs.NewHadesLogConsumer(nc)
	require.NoError(t, err, "create log consumer")

	aggregator := NewLogAggregator(ctx, consumer, AggregatorConfig{
		BatchSize:  100,
		Retention:  time.Hour,
		MaxJobLogs: 1000,
		// No APIendpoint: SendJobLogs is a no-op, we assert on the in-memory store.
	})

	dlm := &DynamicLogManager{
		nc:            nc,
		logConsumer:   consumer,
		logAggregator: aggregator,
		watchers:      make(map[string]watcherState),
	}

	const jobID = "race-job"
	containerIDs := []string{"buildjob-step-1", "buildjob-step-2", "buildjob-step-3", "buildjob-finalizer"}

	// Publish every container's log batch to JetStream first, mirroring the operator
	// publishing all logs before announcing "succeeded". js.Publish waits for the server
	// ack, so all batches are durably stored before we drive the status transitions.
	for i, cid := range containerIDs {
		err := producer.PublishJobLog(ctx, buildlogs.Log{
			JobID:       jobID,
			ContainerID: cid,
			Logs: []buildlogs.LogEntry{
				{Timestamp: time.Now(), Message: fmt.Sprintf("log for %s", cid), OutputStream: "stdout"},
				{Timestamp: time.Now(), Message: fmt.Sprintf("line 2 of container %d", i), OutputStream: "stdout"},
			},
		})
		require.NoError(t, err, "publish log for %s", cid)
	}

	// Drive the running -> succeeded path. handleJobSucceeded blocks until the graceful
	// drain has aggregated every in-flight batch, so the assertions below are deterministic.
	dlm.handleJobRunning(ctx, jobID)
	dlm.handleJobSucceeded(ctx, jobID)

	logs := aggregator.GetJobLogs(jobID)

	got := make([]string, 0, len(logs))
	for _, l := range logs {
		got = append(got, l.ContainerID)
	}
	sort.Strings(got)

	want := append([]string(nil), containerIDs...)
	sort.Strings(want)

	require.ElementsMatch(t, want, got,
		"all container logs must be aggregated before the job is completed; got %v", got)

	for _, l := range logs {
		require.Len(t, l.Logs, 2, "each container's two log entries must survive the drain (container %s)", l.ContainerID)
	}
}

// TestStopWatchingWithoutWatcherIsNoop guards the case where a terminal status arrives
// for a job that was never running: stopWatchingJobLogs must not panic or block.
func TestStopWatchingWithoutWatcherIsNoop(t *testing.T) {
	dlm := &DynamicLogManager{
		watchers: make(map[string]watcherState),
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		dlm.stopWatchingJobLogs(context.Background(), "unknown-job")
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stopWatchingJobLogs blocked for an unknown job")
	}
}
