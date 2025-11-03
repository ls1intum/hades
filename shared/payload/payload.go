package payload

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultPriority is the default priority level for jobs when not specified
	DefaultPriority = 3
)

// RESTPayload represents the HTTP request payload for creating a new job.
// It extends QueuePayload with priority information.
type RESTPayload struct {
	Priority int `json:"priority"` // Priority level (1=low, 2=medium, 3+=high)
	QueuePayload
}

// QueuePayload represents a job to be processed by the Hades system.
// It contains all information needed to execute a multi-step job.
type QueuePayload struct {
	ID        uuid.UUID         `json:"id"`                      // Unique job identifier
	Name      string            `json:"name" binding:"required"` // Human-readable job name
	Timestamp time.Time         `json:"timestamp"`               // Job creation timestamp
	Metadata  map[string]string `json:"metadata"`                // Additional job-level metadata
	Steps     []Step            `json:"steps"`                   // Ordered list of steps to execute
}

// Step represents a single execution step in a job.
// Each step runs in its own container with the specified image and resources.
type Step struct {
	ID          int               `json:"id"`           // Step execution order (starts at 1)
	Name        string            `json:"name"`         // Human-readable step name
	Image       string            `json:"image"`        // Container image to use (e.g., "alpine:latest")
	Script      string            `json:"script"`       // Shell script to execute in the container
	Metadata    map[string]string `json:"metadata"`     // Step-specific environment variables and metadata
	CPULimit    uint              `json:"cpu_limit"`    // CPU limit in millicores (e.g., 1000 = 1 CPU core)
	MemoryLimit string            `json:"memory_limit"` // Memory limit (e.g., "512M", "2G")
}

// IDString returns the step ID as a string.
// This is used for generating unique names in Kubernetes resources.
func (s Step) IDString() string {
	return strconv.Itoa(s.ID)
}
