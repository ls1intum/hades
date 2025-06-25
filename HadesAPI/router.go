package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	log "github.com/sirupsen/logrus"
)

func setupRouter(auth_key string) *gin.Engine {
	r := gin.New()
	r.Use(gin.ErrorLogger())
	r.Use(gin.Recovery())
	if auth_key == "" {
		log.Warn("No auth key set")
	} else {
		log.Info("Auth key set")
		r.Use(gin.BasicAuth(gin.Accounts{
			"hades": auth_key,
		}))
	}

	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)
	return r
}

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

	// Check whether the request is valid
	for _, step := range payload.QueuePayload.Steps {
		if step.MemoryLimit != "" {
			if _, err := utils.ParseMemoryLimit(step.MemoryLimit); err != nil {
				log.WithError(err).Error("Failed to parse RAM limit")
				c.String(http.StatusBadRequest, "Failed to parse RAM limit")
				return
			}
		}
	}

	payload.QueuePayload.ID = utils.GenerateUUID()
	log.Debug("Received build request ", payload)

	queuePrio := utils.PrioFromInt(payload.Priority)

	err := HadesProducer.EnqueueJobWithPriority(c.Request.Context(), payload.QueuePayload, queuePrio)
	if err != nil {
		log.WithError(err).Error("Failed to enqueue job")
		c.String(http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	// Get the priority level for subject routing
	// queuePriority := utils.PrioFromInt(payload.Priority)

	log.Printf("Successfully enqueued job: %s", payload.QueuePayload.ID.String())
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully enqueued job",
		"job_id":  payload.QueuePayload.ID.String(),
	})
}
