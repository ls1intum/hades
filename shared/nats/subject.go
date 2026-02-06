package nats

import (
	"fmt"

	hades "github.com/ls1intum/hades/shared"
)

// natsSubjectBase is the base NATS subject for all job messages
const natsSubjectBase = "hades.jobs"

// prioritySubject returns the NATS subject for a given priority level.
func prioritySubject(p hades.Priority) string {
	return fmt.Sprintf("%s.%s", natsSubjectBase, p)
}
