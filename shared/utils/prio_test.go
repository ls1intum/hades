package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrioFromInt(t *testing.T) {
	tests := []struct {
		priority int
		expected string
	}{
		{10, "critical"},
		{5, "critical"},
		{4, "high"},
		{3, "normal"},
		{2, "low"},
		{1, "minimal"},
		{0, "minimal"},
		{-1, "minimal"},
	}

	for _, test := range tests {
		got := PrioFromInt(test.priority)
		assert.Equal(t, test.expected, got)
	}
}
