package utils

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGigabyte(t *testing.T) {
	got, err := ParseMemoryLimit("1G")
	assert.Nil(t, err)
	assert.Equal(t, int64(1073741824), got)

	got, err = ParseMemoryLimit("2g")
	assert.Nil(t, err)
	assert.Equal(t, int64(2147483648), got)
}

func TestMegabyte(t *testing.T) {
	got, err := ParseMemoryLimit("1M")
	assert.Nil(t, err)
	assert.Equal(t, int64(1048576), got)

	got, err = ParseMemoryLimit("2m")
	assert.Nil(t, err)
	assert.Equal(t, int64(2097152), got)
}

func TestUnknownUnit(t *testing.T) {
	_, err := ParseMemoryLimit("1T")
	assert.NotNil(t, err)
	assert.Equal(t, "unknown unit: T", err.Error())
}

func TestInvalidNumber(t *testing.T) {
	_, err := ParseMemoryLimit("abc")
	assert.NotNil(t, err)
	assert.IsType(t, &strconv.NumError{}, err)
}
