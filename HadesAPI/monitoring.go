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

	// Remove sensitive metadata from job steps
	for i := range job.Steps {
		job.Steps[i].Metadata = make(map[string]string)
	}

	json_tmp, _ := json.Marshal(job)
	return string(json_tmp)
}
