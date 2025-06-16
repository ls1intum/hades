package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrioFromInt(t *testing.T) {
	tests := []struct {
		priority int
		expected Priority
	}{
		{10, HighPriority},
		{5, HighPriority},
		{4, HighPriority},
		{3, HighPriority},
		{2, MediumPriority},
		{1, LowPriority},
		{0, LowPriority},
		{-1, LowPriority},
	}

	for _, test := range tests {
		got := PrioFromInt(test.priority)
		assert.Equal(t, test.expected, got)
	}
}
