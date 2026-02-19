package main

import (
	"encoding/json"

	"log/slog"

	"github.com/ls1intum/hades/shared/payload"
)

// PayloadInput constrains the accepted input types for SafePayloadFormat.
type PayloadInput interface {
	payload.QueuePayload | []byte | string
}

// SafePayloadFormat formats a job payload for display, removing sensitive metadata.
// It accepts a payload.QueuePayload, []byte, or string.
func SafePayloadFormat[T PayloadInput](input T) string {
	switch v := any(input).(type) {
	case payload.QueuePayload:
		return sanitizeAndMarshal(v)
	case []byte:
		var job payload.QueuePayload
		if err := json.Unmarshal(v, &job); err != nil {
			slog.Error("Failed to unmarshal payload bytes", "error", err)
			return ""
		}
		return sanitizeAndMarshal(job)
	case string:
		return SafePayloadFormat([]byte(v))
	}
	return ""
}

func sanitizeAndMarshal(job payload.QueuePayload) string {
	job.Metadata = make(map[string]string)
	// Copy the steps slice so we don't mutate the caller's underlying array.
	stepsCopy := make([]payload.Step, len(job.Steps))
	copy(stepsCopy, job.Steps)
	for i := range stepsCopy {
		stepsCopy[i].Metadata = make(map[string]string)
	}
	job.Steps = stepsCopy
	jsonTmp, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal sanitized payload", "error", err)
		return ""
	}
	return string(jsonTmp)
}
