package main

import (
	"encoding/json"
	"net/url"

	"log/slog"

	"github.com/hades-scheduler/hades/shared/payload"
	"github.com/hades-scheduler/hades/shared/redact"
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
	// keeps keys but masks sensitive values. Drop returns a deep copy, so the
	// caller's payload is left untouched.
	job = redact.Drop(job)
	// Strip any credentials/query/fragment from the callback URL so tokens
	// carried there don't leak into logs.
	if job.CallbackURL != "" {
		if u, err := url.Parse(job.CallbackURL); err == nil {
			u.User = nil
			u.RawQuery = ""
			u.Fragment = ""
			job.CallbackURL = u.String()
		}
	}
	jsonTmp, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal sanitized payload", "error", err)
		return ""
	}
	return string(jsonTmp)
}
