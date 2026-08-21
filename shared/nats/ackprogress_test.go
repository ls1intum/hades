package nats

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAckProgressInterval(t *testing.T) {
	tests := []struct {
		name    string
		ackWait time.Duration
		want    time.Duration
	}{
		{"default ack wait is capped", DefaultAckWait, maxAckProgressInterval},
		{"long ack wait is capped", 1 * time.Hour, maxAckProgressInterval},
		{"short ack wait scales down", 3 * time.Second, 1 * time.Second},
		{"tiny ack wait hits the floor", 10 * time.Millisecond, minAckProgressInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ackProgressInterval(tt.ackWait))
			// The heartbeat must always be well below AckWait, otherwise it
			// cannot reset the ack timer in time.
			if tt.ackWait > minAckProgressInterval {
				assert.Less(t, ackProgressInterval(tt.ackWait), tt.ackWait)
			}
		})
	}
}

func TestConsumerConfigWithDefaults(t *testing.T) {
	cfg := ConsumerConfig{}.withDefaults()
	assert.Equal(t, uint(1), cfg.Concurrency)
	assert.Equal(t, DefaultAckWait, cfg.AckWait)
	assert.Equal(t, DefaultMaxDeliver, cfg.MaxDeliver)

	// MaxDeliver must never end up unlimited (0), which is the JetStream default.
	assert.Positive(t, ConsumerConfig{MaxDeliver: -1}.withDefaults().MaxDeliver)

	// Explicit values are kept.
	custom := ConsumerConfig{Concurrency: 4, AckWait: 5 * time.Second, MaxDeliver: 2}.withDefaults()
	assert.Equal(t, ConsumerConfig{Concurrency: 4, AckWait: 5 * time.Second, MaxDeliver: 2}, custom)
}

func TestStartAckProgressSignalsRepeatedly(t *testing.T) {
	var calls atomic.Int64
	stop := startAckProgress(func() error {
		calls.Add(1)
		return nil
	}, 10*time.Millisecond, "job-1")

	assert.Eventually(t, func() bool { return calls.Load() >= 3 }, 2*time.Second, 5*time.Millisecond,
		"expected repeated in-progress signals while the job is running")

	stop()
}

// TestStartAckProgressStopIsFinalAndIdempotent guards the two failure modes of
// the heartbeat goroutine: leaking past the job, and signalling in-progress on a
// message that has already been acked or naked.
func TestStartAckProgressStopIsFinalAndIdempotent(t *testing.T) {
	var calls atomic.Int64
	stop := startAckProgress(func() error {
		calls.Add(1)
		return nil
	}, time.Millisecond, "job-1")

	assert.Eventually(t, func() bool { return calls.Load() > 0 }, 2*time.Second, time.Millisecond)

	stop()
	after := calls.Load()

	// Calling stop twice (defer plus explicit call in processJob) must not panic
	// on a closed channel or block.
	require.NotPanics(t, stop)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, after, calls.Load(), "in-progress must not be signalled after stop returns")
}

func TestStartAckProgressSurvivesSignalErrors(t *testing.T) {
	var calls atomic.Int64
	stop := startAckProgress(func() error {
		calls.Add(1)
		return errors.New("connection closed")
	}, time.Millisecond, "job-1")
	defer stop()

	// A failed heartbeat is logged, not fatal: the next tick tries again.
	assert.Eventually(t, func() bool { return calls.Load() >= 3 }, 2*time.Second, time.Millisecond)
}
