package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindMemoryLimit(t *testing.T) {
	tests := []struct {
		name     string
		x        string
		y        string
		expected int64
	}{
		{"both empty", "", "", 0},
		{"x valid, y empty", "1G", "", 1073741824},
		{"x empty, y valid", "", "2M", 2097152},
		{"both valid, y smaller", "1G", "2M", 2097152},
		{"both valid, x smaller", "512M", "1G", 536870912},
		{"x invalid, y valid", "invalid", "2M", 2097152},
		{"x valid, y invalid", "1G", "invalid", 1073741824},
		{"both invalid", "invalid1", "invalid2", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindMemoryLimit(tt.x, tt.y)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFindLimit(t *testing.T) {
	t.Run("integers", func(t *testing.T) {
		tests := []struct {
			name     string
			x        int
			y        int
			expected int
		}{
			{"both zero", 0, 0, 0},
			{"x zero", 0, 10, 10},
			{"y zero", 10, 0, 10},
			{"both non-zero, x smaller", 5, 10, 5},
			{"both non-zero, y smaller", 10, 5, 5},
			{"equal values", 7, 7, 7},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := FindLimit(tt.x, tt.y)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("floats", func(t *testing.T) {
		tests := []struct {
			name     string
			x        float64
			y        float64
			expected float64
		}{
			{"both zero", 0.0, 0.0, 0.0},
			{"x zero", 0.0, 5.5, 5.5},
			{"y zero", 3.3, 0.0, 3.3},
			{"both non-zero", 2.5, 7.8, 2.5},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := FindLimit(tt.x, tt.y)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("strings", func(t *testing.T) {
		tests := []struct {
			name     string
			x        string
			y        string
			expected string
		}{
			{"both empty", "", "", ""},
			{"x empty", "", "hello", "hello"},
			{"y empty", "world", "", "world"},
			{"both non-empty", "apple", "banana", "apple"}, // lexicographically smaller
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := FindLimit(tt.x, tt.y)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}
