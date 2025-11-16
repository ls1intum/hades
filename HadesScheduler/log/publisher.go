package log

import (
	"context"

	logs "github.com/ls1intum/hades/shared/buildlogs"
	status "github.com/ls1intum/hades/shared/buildstatus"
)

// NoopPublisher is a Publisher implementation that does nothing.
// Use this as a default when no actual publishing is needed.
type NoopPublisher struct{}

// NewNoopPublisher creates a new no-op publisher
func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

// PublishLog does nothing and always returns nil
func (np *NoopPublisher) PublishJobLog(ctx context.Context, buildJobLog logs.Log) error {
	return nil
}

// PublishJobStatus does nothing and always returns nil
func (np *NoopPublisher) PublishJobStatus(ctx context.Context, status status.JobStatus, jobID string) error {
	return nil
}
