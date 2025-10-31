package utils

import (
	"context"
	"log/slog"
)

// Logger returns the default structured logger
func Logger() *slog.Logger {
	return slog.Default()
}

// LoggerWithContext returns a logger with context values attached
func LoggerWithContext(ctx context.Context) *slog.Logger {
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
