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

// stripURLSecrets removes userinfo, query, and fragment from raw so credentials
// or tokens carried there never reach a log line. An unparseable value is
// returned unchanged only when it is empty; otherwise the original is kept as-is
// because url.Parse is extremely permissive and rarely fails.
func stripURLSecrets(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func sanitizeAndMarshal(job payload.QueuePayload) string {
	// Drop all metadata (keys and values) before logging; key names add no
	// value to log output. The dashboard uses redact.Redactor instead, which
	// keeps keys but masks sensitive values. Drop returns a deep copy, so the
	// caller's payload is left untouched.
	job = redact.Drop(job)
	// Strip any credentials/query/fragment from the callback URLs so tokens
	// carried there don't leak into logs.
	job.CallbackURL = stripURLSecrets(job.CallbackURL)
	job.StatusCallback = stripURLSecrets(job.StatusCallback)
	jsonTmp, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal sanitized payload", "error", err)
		return ""
	}
	return string(jsonTmp)
}
