package main

import (
	"encoding/json"

	"github.com/ls1intum/hades/shared/payload"
	log "github.com/sirupsen/logrus"
)

// SafePayloadFormatter formats a job payload for display, removing sensitive metadata
func SafePayloadFormatter(bytes []byte) string {
	var job payload.QueuePayload
	if err := json.Unmarshal(bytes, &job); err != nil {
		log.WithError(err).Error("Failed to unmarshal task payload")
		return ""
	}

	job.Metadata = make(map[string]string)
	// Remove sensitive metadata from job steps
	for i := range job.Steps {
		job.Steps[i].Metadata = make(map[string]string)
	}

	jsonTmp, err := json.Marshal(job)
	if err != nil {
		log.WithError(err).Error("Failed to marshal sanitized payload")
		return ""
	}
	return string(jsonTmp)
}
