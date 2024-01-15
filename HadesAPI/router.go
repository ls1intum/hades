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
	err := JobQueue.Enqueue(c.Request.Context(), payload.QueuePayload, uint8(payload.Priority))
	if err != nil {
		log.WithError(err).Error("Failed to enqueue build")
		c.String(http.StatusInternalServerError, "Failed to enqueue build")
		return
	}
}

func MonitoringQueue(c *gin.Context) {
	state := MonitorClient.GetQueueState()
	c.JSON(http.StatusOK, state)
}
