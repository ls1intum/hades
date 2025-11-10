package log

import (
	"context"

	logs "github.com/ls1intum/hades/shared/buildlogs"
)

// Publisher defines the interface for publishing logs and job status updates
type Publisher interface {
	PublishLog(ctx context.Context, buildJobLog logs.Log) error
	PublishJobStatus(ctx context.Context, status logs.JobStatus, jobID string) error
}

// NoopPublisher is a Publisher implementation that does nothing.
// Use this as a default when no actual publishing is needed.
type NoopPublisher struct{}

// NewNoopPublisher creates a new no-op publisher
func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

// PublishLog does nothing and always returns nil
func (np *NoopPublisher) PublishLog(ctx context.Context, buildJobLog logs.Log) error {
	return nil
}

// PublishJobStatus does nothing and always returns nil
func (np *NoopPublisher) PublishJobStatus(ctx context.Context, status logs.JobStatus, jobID string) error {
	return nil
}
