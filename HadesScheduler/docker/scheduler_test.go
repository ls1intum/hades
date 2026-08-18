package docker

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hades-scheduler/hades/hadesScheduler/log"
	"github.com/hades-scheduler/hades/shared/buildlogs"
	"github.com/hades-scheduler/hades/shared/payload"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
)

// capturingPublisher records every Log handed to it so tests can assert on the
// output that was demultiplexed from the container's log stream.
type capturingPublisher struct {
	mu   sync.Mutex
	logs []buildlogs.Log
}

func (c *capturingPublisher) PublishJobLog(_ context.Context, jobLog buildlogs.Log) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, jobLog)
	return nil
}

// messages returns every log line published across all containers.
func (c *capturingPublisher) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var out []string
	for _, jobLog := range c.logs {
		for _, entry := range jobLog.Logs {
			out = append(out, entry.Message)
		}
	}
	return out
}

// newTestScheduler builds a Scheduler against the daemon described by the
// environment, skipping the test when no Docker daemon is reachable.
func newTestScheduler(t *testing.T, publisher buildlogs.LogPublisher) *Scheduler {
	t.Helper()

	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("no Docker client available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		t.Skipf("no Docker daemon reachable: %v", err)
	}

	return &Scheduler{
		cli: cli,
		Options: Options{
			// alpine ships /bin/sh, not bash.
			scriptExecutor:      "/bin/sh -c",
			containerAutoremove: false,
		},
		logPublisher:    publisher,
		statusPublisher: log.NewNoopPublisher(),
	}
}

const testImage = "alpine:3.22"

// TestScheduleJobSharesVolumeBetweenSteps runs a real two-step job end to end.
// It covers every Docker API call the executor makes: volume create/remove,
// image pull, container create/start/wait/logs/remove - and asserts that the
// per-job shared volume actually carries state from one step to the next.
func TestScheduleJobSharesVolumeBetweenSteps(t *testing.T) {
	publisher := &capturingPublisher{}
	scheduler := newTestScheduler(t, publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	job := payload.QueuePayload{
		ID:   uuid.New(),
		Name: "shared-volume-job",
		Steps: []payload.Step{
			{ID: 1, Image: testImage, Script: "echo hello-from-step-one > /shared/marker"},
			{ID: 2, Image: testImage, Script: "cat /shared/marker"},
		},
	}

	require.NoError(t, scheduler.ScheduleJob(ctx, job))

	// Step two can only print the marker if it saw step one's write, so this
	// asserts both the shared volume and the log demultiplexing path.
	require.Contains(t, strings.Join(publisher.messages(), "\n"), "hello-from-step-one")
}

// TestScheduleJobFailsOnNonZeroExit asserts a failing step aborts the job.
func TestScheduleJobFailsOnNonZeroExit(t *testing.T) {
	publisher := &capturingPublisher{}
	scheduler := newTestScheduler(t, publisher)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	job := payload.QueuePayload{
		ID:   uuid.New(),
		Name: "failing-job",
		Steps: []payload.Step{
			{ID: 1, Image: testImage, Script: "echo boom >&2; exit 3"},
			{ID: 2, Image: testImage, Script: "echo should-not-run"},
		},
	}

	err := scheduler.ScheduleJob(ctx, job)
	require.Error(t, err)
	require.Contains(t, err.Error(), "container exited with status 3")

	// The second step must not have run, and stderr from the first must still
	// have been captured and published.
	joined := strings.Join(publisher.messages(), "\n")
	require.Contains(t, joined, "boom")
	require.NotContains(t, joined, "should-not-run")
}

// TestPullImagesReportsErrorForMissingImage asserts that errors the daemon
// reports in-band on the pull stream surface as errors rather than being
// silently drained.
func TestPullImagesReportsErrorForMissingImage(t *testing.T) {
	scheduler := newTestScheduler(t, &capturingPublisher{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err := pullImages(ctx, scheduler.cli, "hades-scheduler/hades-no-such-image:definitely-missing")
	require.Error(t, err)
}
