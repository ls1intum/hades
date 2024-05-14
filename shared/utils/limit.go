package utils

import log "github.com/sirupsen/logrus"

func FindLimit(x, y int) int {
	if x == 0 {
		return y
	}
	if y == 0 {
		return x
	}
	if x < y {
		return x
	}
	return y
}

// Gets two memory limits and returns the smaller one as number of bytes
func FindMemoryLimit(x, y string) int {
	// Check the global RAM Limit
	var first_ram_limit int64
	if x != "" {
		bytes, err := ParseMemoryLimit(x)
		if err != nil {
			log.WithError(err).Errorf("Failed to parse global RAM limit %s", x)
			first_ram_limit = 0
		} else {
			first_ram_limit = bytes
		}
	}
	var second_ram_limit int64
	if y != "" {
		bytes, err := ParseMemoryLimit(y)
		if err != nil {
			log.WithError(err).Errorf("Failed to parse step RAM limit %s", y)
			second_ram_limit = 0
		} else {
			second_ram_limit = bytes
		}
	}
	return FindLimit(int(second_ram_limit), int(first_ram_limit))
}
