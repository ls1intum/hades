package payload

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStep_IDString(t *testing.T) {
	tests := []struct {
		name     string
		stepID   int
		expected string
	}{
		{"single digit", 1, "1"},
		{"double digit", 42, "42"},
		{"zero", 0, "0"},
		{"large number", 9999, "9999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := Step{ID: tt.stepID}
			result := step.IDString()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueuePayload_Construction(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	metadata := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	steps := []Step{
		{
			ID:          1,
			Name:        "Build",
			Image:       "golang:1.21",
			Script:      "go build",
			Metadata:    map[string]string{"ENV": "production"},
			CPULimit:    1000,
			MemoryLimit: "512M",
		},
		{
			ID:          2,
			Name:        "Test",
			Image:       "golang:1.21",
			Script:      "go test",
			Metadata:    map[string]string{"ENV": "test"},
			CPULimit:    500,
			MemoryLimit: "256M",
		},
	}

	payload := QueuePayload{
		ID:        id,
		Name:      "Test Job",
		Timestamp: now,
		Metadata:  metadata,
		Steps:     steps,
	}

	assert.Equal(t, id, payload.ID)
	assert.Equal(t, "Test Job", payload.Name)
	assert.Equal(t, now, payload.Timestamp)
	assert.Equal(t, metadata, payload.Metadata)
	assert.Len(t, payload.Steps, 2)
	assert.Equal(t, steps[0].Name, payload.Steps[0].Name)
	assert.Equal(t, steps[1].Name, payload.Steps[1].Name)
}

func TestRESTPayload_DefaultPriority(t *testing.T) {
	payload := RESTPayload{
		Priority: DefaultPriority,
		QueuePayload: QueuePayload{
			ID:   uuid.New(),
			Name: "Test",
		},
	}

	assert.Equal(t, DefaultPriority, payload.Priority)
	assert.Equal(t, 3, payload.Priority)
}

func TestStep_AllFields(t *testing.T) {
	metadata := map[string]string{
		"GIT_URL":    "https://github.com/user/repo",
		"GIT_BRANCH": "main",
	}

	step := Step{
		ID:          1,
		Name:        "Clone Repository",
		Image:       "alpine/git:latest",
		Script:      "git clone $GIT_URL",
		Metadata:    metadata,
		CPULimit:    2000,
		MemoryLimit: "1G",
	}

	assert.Equal(t, 1, step.ID)
	assert.Equal(t, "Clone Repository", step.Name)
	assert.Equal(t, "alpine/git:latest", step.Image)
	assert.Equal(t, "git clone $GIT_URL", step.Script)
	assert.Equal(t, metadata, step.Metadata)
	assert.Equal(t, uint(2000), step.CPULimit)
	assert.Equal(t, "1G", step.MemoryLimit)
	assert.Equal(t, "1", step.IDString())
}
