// Package buildlogs defines the build-log wire types and the publisher/consumer
// plumbing that moves per-job container logs across the system. A Log is
// published to NATS JetStream on the subject "hades.logs.<jobID>" by the
// component running a job's containers, aggregated by HadesLogManager, and
// finally forwarded over HTTP to the Artemis adapter. Because these types are
// serialized to JSON at both hops, their field tags are a cross-component
// contract - change them in lockstep with any consumer.
package buildlogs

import "time"

// LogEntry is a single line of container output at a point in time.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"` // When the line was emitted
	Message   string    `json:"message"`   // The log line (without trailing newline)
	// OutputStream identifies the source stream, "stdout" or "stderr".
	OutputStream string `json:"output_stream"`
}

// Log holds all captured output for a single container of a job. One Log is
// produced per container, so a multi-step job yields one Log per step; the
// ordering by step matters to downstream consumers (the Artemis adapter reads
// the execute step's logs by index).
type Log struct {
	JobID       string     `json:"job_id"`       // Job the container belongs to
	ContainerID string     `json:"container_id"` // Container/step identifier
	Logs        []LogEntry `json:"logs"`         // Ordered log entries for this container
}
