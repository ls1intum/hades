package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindMemoryLimit(t *testing.T) {
	// Test case 1: x and y are empty
	result := FindMemoryLimit("", "")
	assert.Equal(t, 0, result)

	// Test case 2: x is valid, y is empty
	result = FindMemoryLimit("1G", "")
	assert.Equal(t, 1073741824, result)

	// Test case 3: x is empty, y is valid
	result = FindMemoryLimit("", "2M")
	assert.Equal(t, 2097152, result)

	// Test case 4: x and y are valid
	result = FindMemoryLimit("1G", "2M")
	assert.Equal(t, 2097152, result)

	// Test case 5: x is invalid, y is valid
	result = FindMemoryLimit("abc", "2M")
	assert.Equal(t, 2097152, result)

	// Test case 6: x is valid, y is invalid
	result = FindMemoryLimit("1G", "xyz")
	assert.Equal(t, 1073741824, result)
}
