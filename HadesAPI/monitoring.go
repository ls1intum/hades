package main

import (
	"encoding/json"

	"github.com/hibiken/asynqmon"
	"github.com/ls1intum/hades/shared/payload"
	log "github.com/sirupsen/logrus"
)

var MetadataObfuscator = asynqmon.PayloadFormatterFunc(func(_ string, bytes []byte) string {
	var job payload.QueuePayload
	if err := json.Unmarshal(bytes, &job); err != nil {
		log.WithError(err).Error("Failed to unmarshal task payload")
		return ""
	}
	for i := range job.Steps {
		job.Steps[i].Metadata = make(map[string]string)
	}

	json_tmp, _ := json.Marshal(job)
	return string(json_tmp)
})
