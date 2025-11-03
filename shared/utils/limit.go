package utils

import (
	"cmp"
	"log/slog"
)

// FindLimit returns the minimum of two ordered values, treating zero values as "no limit".
// If either value is zero, it returns the non-zero value.
// If both are non-zero, it returns the minimum.
func FindLimit[T cmp.Ordered](x, y T) T {
	var zero T
	if x == zero {
		return y
	}
	if y == zero {
		return x
	}
	return min(x, y)
}

// FindMemoryLimit parses two memory limit strings and returns the effective limit in bytes.
// It returns the smaller of the two limits, treating empty strings or parse errors as "no limit".
// This is useful for determining the effective memory limit when both global and local limits exist.
func FindMemoryLimit(x, y string) int64 {
	var firstLimit int64
	if x != "" {
		bytes, err := ParseMemoryLimit(x)
		if err != nil {
			slog.With("error", err).Error("Failed to parse first memory limit", "limit", x)
		} else {
			firstLimit = bytes
		}
	}

	var secondLimit int64
	if y != "" {
		bytes, err := ParseMemoryLimit(y)
		if err != nil {
			slog.With("error", err).Error("Failed to parse second memory limit", "limit", y)
		} else {
			secondLimit = bytes
		}
	}

	return FindLimit(firstLimit, secondLimit)
}
