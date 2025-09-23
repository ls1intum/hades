package utils

import "fmt"

type Priority string

const (
	HighPriority   Priority = "high"
	MediumPriority Priority = "medium"
	LowPriority    Priority = "low"
)

// PrioritySubject returns the NATS subject for a given priority
func PrioritySubject(p Priority) string {
	return fmt.Sprintf("%s.%s", NatsJobSubject, p)
}

func PrioFromInt(priority int) Priority {
	switch {
	case priority >= 3:
		return HighPriority
	case priority <= 1:
		return LowPriority
	default:
		return MediumPriority
	}
}
