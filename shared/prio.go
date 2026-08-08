package hades

// Priority represents job priority levels for queue management.
type Priority string

const (
	// HighPriority represents high-priority jobs (priority >= 3)
	HighPriority Priority = "high"
	// MediumPriority represents medium-priority jobs (priority == 2)
	MediumPriority Priority = "medium"
	// LowPriority represents low-priority jobs (priority <= 1)
	LowPriority Priority = "low"
)

var (
	// Priorities defines the order in which job queues are checked (high to low)
	Priorities = []Priority{HighPriority, MediumPriority, LowPriority}
)

const (
	// MetadataKeyPriority is the job-metadata key under which the numeric job
	// priority is propagated from the scheduler to the executors. The value is
	// the stringified integer from PriorityToInt. The Kubernetes executor and
	// the operator also surface this key as a Pod/Job label of the same name.
	// This string is an implicit cross-component contract; always reference this
	// constant rather than the literal so writer and readers stay in sync.
	MetadataKeyPriority = "hades.tum.de/priority"
	// MetadataKeyPriorityName is the job-metadata key carrying the human-readable
	// priority name ("high"/"medium"/"low"). Written alongside MetadataKeyPriority.
	MetadataKeyPriorityName = "hades.tum.de/priorityName"
)

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
