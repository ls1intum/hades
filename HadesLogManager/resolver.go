package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/nats-io/nats.go/jetstream"
)

// jobCallbackResolver resolves the per-job callback URL for a given job ID.
type jobCallbackResolver interface {
	CallbackURL(ctx context.Context, jobID string) (string, error)
}

// kvCallbackResolver resolves the callback URL by reading the job payload from
// the HADES_JOBS JetStream key-value store, where HadesAPI stores every job
// keyed by its ID. This is the same bucket the scheduler consumes from.
type kvCallbackResolver struct {
	kv jetstream.KeyValue
}

// newKVCallbackResolver creates a resolver backed by the given KV store.
func newKVCallbackResolver(kv jetstream.KeyValue) *kvCallbackResolver {
	return &kvCallbackResolver{kv: kv}
}

// CallbackURL returns the callback URL stored on the job payload for jobID.
// A missing key or bucket yields ("", nil) so callers treat it as "no callback
// configured" rather than an error. Read or unmarshal failures are returned as
// errors.
func (r *kvCallbackResolver) CallbackURL(ctx context.Context, jobID string) (string, error) {
	entry, err := r.kv.Get(ctx, jobID)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrBucketNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("reading job payload from KV store: %w", err)
	}

	var job payload.QueuePayload
	if err := json.Unmarshal(entry.Value(), &job); err != nil {
		return "", fmt.Errorf("unmarshaling job payload: %w", err)
	}

	return job.CallbackURL, nil
}
