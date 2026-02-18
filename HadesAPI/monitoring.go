package main

import (
	"encoding/json"

	"log/slog"

	"github.com/ls1intum/hades/shared/payload"
)

// SafePayloadFormatter formats a job payload for display, removing sensitive metadata
func SafePayloadFormatter(bytes []byte) string {
	var job payload.QueuePayload
	if err := json.Unmarshal(bytes, &job); err != nil {
		slog.Error("Failed to unmarshal task payload", "error", err)
		return ""
	}

	job.Metadata = make(map[string]string)
	// Remove sensitive metadata from job steps
	for i := range job.Steps {
		job.Steps[i].Metadata = make(map[string]string)
	}

	jsonTmp, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal sanitized payload", "error", err)
		return ""
	}
	return string(jsonTmp)
}
