package utils

import "cmp"

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

// Gets two memory limits and returns the smaller one as number of bytes
func FindMemoryLimit(x, y string) int {
	// Check the global RAM Limit
	var first_ram_limit int64
	if x != "" {
		bytes, err := ParseMemoryLimit(x)
		if err != nil {
			log.With("error", err).Error("Failed to parse global RAM limit", "limit", x)
			first_ram_limit = 0
		} else {
			first_ram_limit = bytes
		}
	}
	var second_ram_limit int64
	if y != "" {
		bytes, err := ParseMemoryLimit(y)
		if err != nil {
			log.With("error", err).Error("Failed to parse step RAM limit", "limit", y)
			second_ram_limit = 0
		} else {
			second_ram_limit = bytes
		}
	}
	return FindLimit(int(second_ram_limit), int(first_ram_limit))
}
