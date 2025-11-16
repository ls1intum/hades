package utils

import (
	"context"

	"github.com/ls1intum/hades/shared/payload"
)

// JobPublisher manages job publishing.
type JobPublisher interface {
	EnqueueJobWithPriority(ctx context.Context, queuePayload payload.QueuePayload, priority Priority) error
}

// JobConsumer manages job consumption.
type JobConsumer interface {
	DequeueJob(ctx context.Context, processing func(payload payload.QueuePayload))
}
