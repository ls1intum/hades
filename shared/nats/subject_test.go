package nats

import (
	"testing"

	hades "github.com/ls1intum/hades/shared"
	"github.com/stretchr/testify/assert"
)

func TestPrioritySubject(t *testing.T) {
	tests := []struct {
		name     string
		priority hades.Priority
		expected string
	}{
		{"high priority subject", hades.HighPriority, "hades.jobs.high"},
		{"medium priority subject", hades.MediumPriority, "hades.jobs.medium"},
		{"low priority subject", hades.LowPriority, "hades.jobs.low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prioritySubject(tt.priority)
			assert.Equal(t, tt.expected, got)
		})
	}
}
