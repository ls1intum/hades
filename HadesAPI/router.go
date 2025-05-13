package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
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

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},                                                // Allow all origins
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}, // Allowed methods
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},          // Allowed headers
		ExposeHeaders:    []string{"Content-Length"},                                   // Expose headers
		AllowCredentials: true,                                                         // Allow credentials
		MaxAge:           12 * time.Hour,                                               // Preflight request cache duration
	}))

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
	// Main endpoint that supports filtering by status query parameter (?status=queued|active|completed|all)
	r.GET("/builds", ListBuilds)
	r.GET("/workers", ListWorkers)
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
		"job_id":  payload.QueuePayload.ID.String(),
	})
}

// ListBuilds returns a list of build jobs with optional filtering by status
func ListBuilds(c *gin.Context) {
	// Get the status query parameter (queued, active, completed, or all)
	status := c.DefaultQuery("status", "all")

	// Create an inspector to look at the queue
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.RedisConfig.Addr,
		Password: cfg.RedisConfig.Pwd,
	})

	// Get all queue names (critical, default, low)
	queues, err := inspector.Queues()
	if err != nil {
		log.WithError(err).Error("Failed to get queues")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queues"})
		return
	}

	allBuilds := make([]map[string]interface{}, 0)

	// For each queue, get tasks based on the requested status
	for _, queueName := range queues {
		var tasks []*asynq.TaskInfo
		var err error

		switch status {
		case "active":
			tasks, err = inspector.ListActiveTasks(queueName)
			if err != nil {
				log.WithError(err).Errorf("Failed to list active tasks in queue %s", queueName)
				continue
			}
		case "completed":
			tasks, err = inspector.ListCompletedTasks(queueName)
			if err != nil {
				log.WithError(err).Errorf("Failed to list completed tasks in queue %s", queueName)
				continue
			}
		case "all":
			// Get all types of tasks
			pendingTasks, err := inspector.ListPendingTasks(queueName)
			if err == nil {
				tasks = append(tasks, pendingTasks...)
			} else {
				log.WithError(err).Errorf("Failed to list pending tasks in queue %s", queueName)
			}

			activeTasks, err := inspector.ListActiveTasks(queueName)
			if err == nil {
				tasks = append(tasks, activeTasks...)
			} else {
				log.WithError(err).Errorf("Failed to list active tasks in queue %s", queueName)
			}

			completedTasks, err := inspector.ListCompletedTasks(queueName)
			if err == nil {
				tasks = append(tasks, completedTasks...)
			} else {
				log.WithError(err).Errorf("Failed to list completed tasks in queue %s", queueName)
			}
		case "queued":
		default:
			tasks, err = inspector.ListPendingTasks(queueName)
			if err != nil {
				log.WithError(err).Errorf("Failed to list pending tasks in queue %s", queueName)
				continue
			}
		}

		// Process tasks
		for _, task := range tasks {
			var queuePayload payload.QueuePayload
			if err := json.Unmarshal(task.Payload, &queuePayload); err != nil {
				// Skip tasks with invalid payload
				log.WithError(err).Warn("Could not unmarshal task payload")
				continue
			}

			// Remove metadata from each step
			for i := range queuePayload.Steps {
				queuePayload.Steps[i].Metadata = make(map[string]string)
			}

			buildInfo := map[string]interface{}{
				"id":        queuePayload.ID.String(),
				"task_id":   task.ID,
				"task_type": task.Type,
				"queue":     queueName,
				"status":    task.State.String(),
				"steps":     queuePayload.Steps,
			}
			allBuilds = append(allBuilds, buildInfo)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"count":  len(allBuilds),
		"builds": allBuilds,
	})
}

// ListWorkers returns information about all servers processing tasks from the queue
func ListWorkers(c *gin.Context) {
	// Create an inspector to look at the queue
	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr:     cfg.RedisConfig.Addr,
		Password: cfg.RedisConfig.Pwd,
	})

	// Get all server information
	servers, err := inspector.Servers()
	if err != nil {
		log.WithError(err).Error("Failed to get workers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get workers"})
		return
	}

	workersInfo := make([]map[string]interface{}, 0)

	// Process server information
	for _, server := range servers {
		workerInfo := map[string]interface{}{
			"id":             server.ID,
			"host":           server.Host,
			"pid":            server.PID,
			"concurrency":    server.Concurrency,
			"started_at":     server.Started,
			"status":         server.Status,
			"queues":         server.Queues,
			"active_workers": len(server.ActiveWorkers),
		}

		// Add information about active workers if any
		if len(server.ActiveWorkers) > 0 {
			activeWorkers := make([]map[string]interface{}, 0)
			for _, worker := range server.ActiveWorkers {
				activeWorker := map[string]interface{}{
					"task_id":    worker.TaskID,
					"task_type":  worker.TaskType,
					"queue":      worker.Queue,
					"started_at": worker.Started,
					"deadline":   worker.Deadline,
				}
				activeWorkers = append(activeWorkers, activeWorker)
			}
			workerInfo["active_tasks"] = activeWorkers
		}

		workersInfo = append(workersInfo, workerInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(workersInfo),
		"workers": workersInfo,
	})
}
