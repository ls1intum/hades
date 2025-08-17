package logs

import "time"

type LogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	OutputStream string    `json:"output_stream"`
}

type Log struct {
	JobID       string     `json:"job_id"`
	ContainerID string     `json:"container_id"`
	Logs        []LogEntry `json:"logs"`
}
