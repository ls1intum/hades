package main

import (
	"encoding/json"

	"log/slog"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/redact"
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
	// Drop all metadata (keys and values) before logging; key names add no
	// value to log output. The dashboard uses redact.Redactor instead, which
	// keeps keys but masks sensitive values.
	jsonTmp, err := json.Marshal(redact.Drop(job))
	if err != nil {
		slog.Error("Failed to marshal sanitized payload", "error", err)
		return ""
	}
	return string(jsonTmp)
}
