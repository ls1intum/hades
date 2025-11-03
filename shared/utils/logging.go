package utils

import (
	"log/slog"
	"os"
)

// setupLogging configures the logging level based on environment
func SetupLogging() {
	if os.Getenv("DEBUG") == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}
}

// Logger returns the default structured logger
func Logger() *slog.Logger {
	return slog.Default()
}

// LoggerWith creates a logger with predefined attributes
func LoggerWith(attrs ...slog.Attr) *slog.Logger {
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return slog.Default().With(args...)
}

// JobLogger creates a logger with job_id attribute
func JobLogger(jobID string) *slog.Logger {
	return slog.Default().With(slog.String("job_id", jobID))
}

// ComponentLogger creates a logger with component attribute
func ComponentLogger(component string) *slog.Logger {
	return slog.Default().With(slog.String("component", component))
}
