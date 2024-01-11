package main

import (
	"net/http"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/gin-gonic/gin"
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
	JobQueue.Enqueue(c.Request.Context(), payload.QueuePayload, uint8(payload.Priority))
}

func MonitoringQueue(c *gin.Context) {
	state := MonitorClient.GetQueueState()
	c.JSON(http.StatusOK, state)
}
