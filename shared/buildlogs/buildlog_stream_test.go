package buildlogs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entry(msg string) LogEntry {
	return LogEntry{Timestamp: time.Unix(0, 0), Message: msg, OutputStream: "stdout"}
}

// The Artemis adapter locates a step's logs positionally (e.g. logs[1] for the
// execution step), so the batcher must preserve one Log per container in
// first-seen order and keep each ContainerID.
func TestContainerLogBatcher_GroupsPerContainerInOrder(t *testing.T) {
	b := newContainerLogBatcher("job-1")
	b.add(Log{JobID: "job-1", ContainerID: "step-1", Logs: []LogEntry{entry("clone a"), entry("clone b")}})
	b.add(Log{JobID: "job-1", ContainerID: "step-2", Logs: []LogEntry{entry("build")}})

	assert.Equal(t, 3, b.size())

	out := b.drain()
	require.Len(t, out, 2, "one Log per container")

	assert.Equal(t, "step-1", out[0].ContainerID)
	assert.Equal(t, "job-1", out[0].JobID)
	require.Len(t, out[0].Logs, 2)
	assert.Equal(t, "clone a", out[0].Logs[0].Message)

	assert.Equal(t, "step-2", out[1].ContainerID)
	require.Len(t, out[1].Logs, 1)
	assert.Equal(t, "build", out[1].Logs[0].Message)
}

func TestContainerLogBatcher_MergesSameContainer(t *testing.T) {
	b := newContainerLogBatcher("job-1")
	b.add(Log{ContainerID: "step-2", Logs: []LogEntry{entry("part 1")}})
	b.add(Log{ContainerID: "step-2", Logs: []LogEntry{entry("part 2")}})

	out := b.drain()
	require.Len(t, out, 1, "entries for the same container merge into one Log")
	require.Len(t, out[0].Logs, 2)
	assert.Equal(t, "part 1", out[0].Logs[0].Message)
	assert.Equal(t, "part 2", out[0].Logs[1].Message)
}

func TestContainerLogBatcher_IgnoresEmptyAndResetsOnDrain(t *testing.T) {
	b := newContainerLogBatcher("job-1")

	b.add(Log{ContainerID: "step-1", Logs: nil}) // no entries -> ignored
	assert.Equal(t, 0, b.size())
	assert.Nil(t, b.drain())

	b.add(Log{ContainerID: "step-1", Logs: []LogEntry{entry("x")}})
	require.Len(t, b.drain(), 1)

	// Second drain returns nothing: the buffer was reset.
	assert.Nil(t, b.drain())
	assert.Equal(t, 0, b.size())
}
