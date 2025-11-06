package buildlogs

import "fmt"

type JobStatus string

const (
	StatusQueued  JobStatus = "queued"
	StatusRunning JobStatus = "running"
	StatusSuccess JobStatus = "success"
	StatusFailed  JobStatus = "failed"
	StatusStopped JobStatus = "stopped"
)

const StatusSubjectFormat = "hades.jobstatus.%s"

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
