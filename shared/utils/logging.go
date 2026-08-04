package utils

import (
	"log/slog"
	"os"
)

// SetupLogging configures the global logging level based on the DEBUG
// environment variable.
func SetupLogging() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}
}
