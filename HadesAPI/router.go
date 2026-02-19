package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	hades "github.com/ls1intum/hades/shared"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
)

func setupRouter(auth_key string) *gin.Engine {
	r := gin.New()
	r.Use(gin.ErrorLogger())
	r.Use(gin.Recovery())
	if auth_key == "" {
		slog.Warn("No auth key set")
	} else {
		slog.Info("Auth key set")
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
		slog.Error("Failed to bind JSON", "error", err)
		c.String(http.StatusBadRequest, "Failed to bind JSON")
		return
	}

	// Check whether the request is valid
	for _, step := range payload.QueuePayload.Steps {
		if step.MemoryLimit != "" {
			if _, err := utils.ParseMemoryLimit(step.MemoryLimit); err != nil {
				slog.Error("Failed to parse RAM limit", "error", err)
				c.String(http.StatusBadRequest, "Failed to parse RAM limit")
				return
			}
		}
	}

	payload.QueuePayload.ID = uuid.New()
	slog.Debug("Received build request ", "payload", SafePayloadFormat(payload.QueuePayload))

	queuePrio := hades.PriorityFromInt(payload.Priority)

	err := HadesProducer.EnqueueJobWithPriority(c.Request.Context(), payload.QueuePayload, queuePrio)
	if err != nil {
		slog.Error("Failed to enqueue job", "error", err)
		c.String(http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	// Get the priority level for subject routing
	// queuePriority := utils.PrioFromInt(payload.Priority)

	slog.Info("Successfully enqueued job", "job_id", payload.QueuePayload.ID.String())
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully enqueued job",
		"job_id":  payload.QueuePayload.ID.String(),
	})
}
