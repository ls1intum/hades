package buildlogs

import (
	"context"

	"github.com/ls1intum/hades/shared/buildstatus"
)

// LogAggregator defines the interface for aggregating and managing job logs
type LogAggregator interface {
	AddLog(log Log)
	FlushJob(jobID string) error
	GetJobLogs(jobID string) []Log
	GetAllJobs() []string
	MarkJobCompleted(jobID string)
	UpdateJobStatus(jobID string, status buildstatus.JobStatus)
	GetJobStatus(jobID string) (buildstatus.JobStatus, error)
}

// LogManager defines the interface for managing job log subscriptions
type LogManager interface {
	StartListening(ctx context.Context) error
}
