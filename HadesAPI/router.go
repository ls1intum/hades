package main

import (
	"encoding/json"
	"net/http"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func AddBuildToQueue(c *gin.Context) {
	var payload payload.RESTPayload
	payload.Priority = 3 // Default priority
	if err := c.ShouldBind(&payload); err != nil {
		log.WithError(err).Error("Failed to bind JSON")
		c.String(http.StatusBadRequest, "Failed to bind JSON")
		return
	}

	log.Debug("Received build request ", payload)
	json_payload, err := json.Marshal(payload.QueuePayload)
	if err != nil {
		log.Fatal(err)
	}

	task := asynq.NewTask(payload.Name, json_payload)
	queuePriority := utils.PrioFromInt(payload.Priority)
	info, err := AsynqClient.Enqueue(task, asynq.Queue(queuePriority))
	if err != nil {
		log.WithError(err).Error("Failed to enqueue build")
		c.String(http.StatusInternalServerError, "Failed to enqueue build")
		return
	}
	log.Printf(" [*] Successfully enqueued task: %+v", info.ID)
}
