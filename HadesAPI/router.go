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

func setupRouter(authKey string, producer hades.JobPublisher) *gin.Engine {
	r := gin.New()
	r.Use(gin.ErrorLogger())
	r.Use(gin.Recovery())
	if authKey == "" {
		slog.Warn("No auth key set")
	} else {
		slog.Info("Auth key set")
		r.Use(gin.BasicAuth(gin.Accounts{
			"hades": authKey,
		}))
	}

	r.GET("/ping", ping)
	r.POST("/build", func(c *gin.Context) {
		addBuildToQueue(c, producer)
	})
	return r
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func addBuildToQueue(c *gin.Context, producer hades.JobPublisher) {
	var p payload.RESTPayload
	p.Priority = 3
	if err := c.ShouldBind(&p); err != nil {
		slog.Error("Failed to bind JSON", "error", err)
		c.String(http.StatusBadRequest, "Failed to bind JSON")
		return
	}

	for _, step := range p.QueuePayload.Steps {
		if step.MemoryLimit != "" {
			if _, err := utils.ParseMemoryLimit(step.MemoryLimit); err != nil {
				slog.Error("Failed to parse RAM limit", "error", err)
				c.String(http.StatusBadRequest, "Failed to parse RAM limit")
				return
			}
		}
	}

	p.QueuePayload.ID = uuid.New()
	slog.Debug("Received build request ", "payload", SafePayloadFormat(p.QueuePayload))

	queuePrio := hades.PriorityFromInt(p.Priority)

	err := producer.EnqueueJobWithPriority(c.Request.Context(), p.QueuePayload, queuePrio)
	if err != nil {
		slog.Error("Failed to enqueue job", "error", err)
		c.String(http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	slog.Info("Successfully enqueued job", "job_id", p.QueuePayload.ID.String())
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully enqueued job",
		"job_id":  p.QueuePayload.ID.String(),
	})
}
