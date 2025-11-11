package buildstatus

import (
	"context"
	"fmt"
)

type JobStatus string

// StatusPublisher defines the interface for publishing status updates to NATS JetStream
type StatusPublisher interface {
	PublishStatus(ctx context.Context, status JobStatus, jobID string) error
}

const (
	StatusQueued  JobStatus = "queued"
	StatusRunning JobStatus = "running"
	StatusSuccess JobStatus = "success"
	StatusFailed  JobStatus = "failed"
	StatusStopped JobStatus = "stopped"
)

const StatusSubjectFormat = "hades.status.%s"

// Optional: Add helper methods
func (js JobStatus) String() string {
	return string(js)
}

func (js JobStatus) Subject() string {
	return fmt.Sprintf(StatusSubjectFormat, js)
}

// Optional: Validation
func (js JobStatus) IsValid() bool {
	switch js {
	case StatusQueued, StatusRunning, StatusSuccess, StatusFailed, StatusStopped:
		return true
	default:
		return false
	}
}
