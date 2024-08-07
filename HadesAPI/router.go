package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
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
	r.GET("/task/:id", GetTaskState)
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
	json_payload, err := json.Marshal(payload.QueuePayload)
	if err != nil {
		log.Fatal(err)
	}

	task := asynq.NewTask(payload.Name, json_payload)
	queuePriority := utils.PrioFromInt(payload.Priority)
	info, err := AsynqClient.Enqueue(
		task,
		asynq.Queue(queuePriority),
		asynq.Retention(time.Duration(cfg.RetentionTime)*time.Minute), // Keep the result for 30 minutes
		asynq.Timeout(time.Duration(cfg.Timeout)*time.Minute),         // Timeout for each queued task
		asynq.MaxRetry(int(cfg.MaxRetries)),                           // Retry times
	)
	if err != nil {
		log.WithError(err).Error("Failed to enqueue build")
		c.String(http.StatusInternalServerError, "Failed to enqueue build")
		return
	}
	log.Printf(" [*] Successfully enqueued job: %+v", info.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully enqueued job",
		"task_id": info.ID,
		"job_id":  payload.QueuePayload.ID,
	})
}

func GetTaskState(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.String(http.StatusBadRequest, "No task ID provided")
		return
	}
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: cfg.RedisConfig.Addr, Password: cfg.RedisConfig.Pwd})
	queues, err := inspector.Queues()
	if err != nil {
		log.WithError(err).Error("Failed to get queues")
		c.String(http.StatusInternalServerError, "Failed to get queues")
		return
	}
	for _, q := range queues {
		taskInfo, err := inspector.GetTaskInfo(q, id)
		if err == nil {
			c.JSON(http.StatusOK, taskInfo.State.String())
			return
		}
	}
	c.String(http.StatusNotFound, "Task not found")
}
