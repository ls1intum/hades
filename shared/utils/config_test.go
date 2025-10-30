package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMemoryLimit_Gigabytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"1 gigabyte uppercase", "1G", 1073741824},
		{"2 gigabytes lowercase", "2g", 2147483648},
		{"10 gigabytes", "10G", 10737418240},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemoryLimit(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseMemoryLimit_Megabytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"1 megabyte uppercase", "1M", 1048576},
		{"2 megabytes lowercase", "2m", 2097152},
		{"512 megabytes", "512M", 536870912},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemoryLimit(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseMemoryLimit_UnknownUnit(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"terabyte", "1T"},
		{"kilobyte", "1K"},
		{"byte", "1B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMemoryLimit(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported memory unit")
		})
	}
}

func TestParseMemoryLimit_InvalidInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{"invalid number", "abcM", "invalid memory value"},
		{"empty string", "", "invalid memory limit format"},
		{"single character", "G", "invalid memory limit format"},
		{"negative value", "-5G", "memory limit cannot be negative"},
		{"no unit", "123", "unsupported memory unit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMemoryLimit(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}
