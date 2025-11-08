package buildlogs

import "fmt"

type JobStatus string

const (
	StatusQueued    JobStatus = "Queued"
	StatusRunning   JobStatus = "Running"
	StatusSucceeded JobStatus = "Succeeded"
	StatusFailed    JobStatus = "Failed"
	StatusStopped   JobStatus = "Stopped"
)

const NatsJobStatusSubject = "hades.jobstatus.%s"

func (js JobStatus) String() string {
	return string(js)
}

func (js JobStatus) Subject() string {
	return fmt.Sprintf(NatsJobStatusSubject, js.String())
}

func (js JobStatus) IsValid() bool {
	switch js {
	case StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusStopped:
		return true
	default:
		return false
	}
}
