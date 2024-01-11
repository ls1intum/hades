package queue

import (
	"context"

	"github.com/Mtze/HadesCI/shared/payload"
)

const maxPriority uint8 = 5

type JobQueue interface {
	Enqueue(ctx context.Context, msg payload.QueuePayload, prio uint8) error
	Dequeue(callback func(job payload.QueuePayload) error) error
	Close()
}
