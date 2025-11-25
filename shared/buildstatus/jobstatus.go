package buildstatus

import (
	"context"
	"fmt"
)

type JobStatus string

// StatusPublisher defines the interface for publishing status updates to NATS JetStream
type StatusPublisher interface {
	PublishJobStatus(ctx context.Context, status JobStatus, jobID string) error
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

// Optional: Validation
func (js JobStatus) IsValid() bool {
	switch js {
	case StatusQueued, StatusRunning, StatusSuccess, StatusFailed, StatusStopped:
		return true
	default:
		return false
	}
}

func StatusSubject(status JobStatus) string {
	return fmt.Sprintf(StatusSubjectFormat, status)
}
