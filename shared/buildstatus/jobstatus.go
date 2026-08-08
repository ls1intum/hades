// Package buildstatus defines the job lifecycle status enum and the NATS
// subjects used to publish status transitions. Status events are published on
// "hades.jobstatus.<Status>" with the job ID as the message payload;
// HadesLogManager subscribes to these to start and stop watching a job's logs.
package buildstatus

import (
	"context"
	"fmt"
)

// JobStatus is a point in a job's lifecycle. Its string value is capitalized and
// is used verbatim as the last token of the status NATS subject (see
// StatusSubjectFormat), so the constants below and any subject matching must stay
// in sync.
type JobStatus string

// StatusPublisher publishes job status transitions to NATS JetStream.
type StatusPublisher interface {
	PublishJobStatus(ctx context.Context, status JobStatus, jobID string) error
}

// The job lifecycle statuses. Queued and Running are transient; Succeeded,
// Failed, and Stopped are terminal.
const (
	StatusQueued    JobStatus = "Queued"
	StatusRunning   JobStatus = "Running"
	StatusSucceeded JobStatus = "Succeeded"
	StatusFailed    JobStatus = "Failed"
	StatusStopped   JobStatus = "Stopped"
)

// StatusSubjectFormat is the NATS subject template for status events; format it
// with a JobStatus (e.g. "hades.jobstatus.Running") via StatusSubject.
const StatusSubjectFormat = "hades.jobstatus.%s"

// String returns the status as its underlying string value.
func (js JobStatus) String() string {
	return string(js)
}

// IsValid reports whether js is one of the defined status constants.
func (js JobStatus) IsValid() bool {
	switch js {
	case StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusStopped:
		return true
	default:
		return false
	}
}

// StatusSubject returns the NATS subject for publishing the given status.
func StatusSubject(status JobStatus) string {
	return fmt.Sprintf(StatusSubjectFormat, status)
}
