package hades

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPriorityFromInt(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected Priority
	}{
		{"very high priority", 10, HighPriority},
		{"high priority upper bound", 5, HighPriority},
		{"high priority lower bound plus one", 4, HighPriority},
		{"high priority lower bound", 3, HighPriority},
		{"medium priority", 2, MediumPriority},
		{"low priority upper bound", 1, LowPriority},
		{"zero priority", 0, LowPriority},
		{"negative priority", -1, LowPriority},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PriorityFromInt(tt.priority)
			assert.Equal(t, tt.expected, got)
		})
	}
}
