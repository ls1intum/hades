package utils

import "fmt"

// Priority represents job priority levels for queue management.
type Priority string

const (
	// HighPriority represents high-priority jobs (priority >= 3)
	HighPriority Priority = "high"
	// MediumPriority represents medium-priority jobs (priority == 2)
	MediumPriority Priority = "medium"
	// LowPriority represents low-priority jobs (priority <= 1)
	LowPriority Priority = "low"

	// natsSubjectBase is the base NATS subject for all job messages
	natsSubjectBase = "hades.jobs"
)

var (
	// priorities defines the order in which job queues are checked (high to low)
	priorities = []Priority{HighPriority, MediumPriority, LowPriority}
)

// PrioritySubject returns the NATS subject for a given priority level.
func PrioritySubject(p Priority) string {
	return fmt.Sprintf("%s.%s", natsSubjectBase, p)
}

// PriorityFromInt converts an integer priority value to a Priority type.
// Values >= 3 are high priority, <= 1 are low priority, and 2 is medium priority.
func PriorityFromInt(priority int) Priority {
	switch {
	case priority >= 3:
		return HighPriority
	case priority <= 1:
		return LowPriority
	default:
		return MediumPriority
	}
}

func PriorityToInt(p Priority) int {
	switch p {
	case HighPriority:
		return 3
	case MediumPriority:
		return 2
	default:
		return 1
	}
}
