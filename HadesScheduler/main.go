package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ls1intum/hades/hadesScheduler/docker"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"log/slog"
)

var NatsConnection *nats.Conn
var NatsJetStream nats.JetStreamContext

type JobScheduler interface {
	ScheduleJob(ctx context.Context, job payload.QueuePayload) error
}

type HadesSchedulerConfig struct {
	Concurrency uint `env:"CONCURRENCY" envDefault:"1"`
	NatsConfig  utils.NatsConfig
}

var HadesConsumer *utils.HadesConsumer

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesSchedulerConfig
	utils.LoadConfig(&cfg)

	var executorCfg utils.ExecutorConfig
	utils.LoadConfig(&executorCfg)
	slog.Debug("Executor config: ", "config", executorCfg)

	// Set up NATS connection
	var err error
	NatsConnection, err = utils.SetupNatsConnection(cfg.NatsConfig)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer NatsConnection.Close()

	HadesConsumer, err = utils.NewHadesConsumer(NatsConnection, cfg.Concurrency)
	if err != nil {
		slog.Error("Failed to create Hades consumer", "error", err)
		os.Exit(1)
	}

	js, err := jetstream.New(NatsConnection)
	if err != nil {
		slog.Error("Failed to create JetStream", "error", err)
		return
	}

	ctx := context.Background()

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "job-status",
		TTL:    time.Hour * 24, // Jobs auto-cleanup after 24h
	})
	if err != nil {
		slog.Error("Failed to create KV store", "error", err)
		return
	}

	var scheduler JobScheduler
	switch executorCfg.Executor {
	case "k8s":
		slog.Info("Started HadesScheduler in Kubernetes mode")
		scheduler = k8s.NewK8sScheduler()

	case "docker":
		slog.Info("Started HadesScheduler in Docker mode")

		dockerScheduler, err := docker.NewDockerScheduler(kv)
		if err != nil {
			slog.Error("Failed to create Docker scheduler", "error", err)
			return
		}
		scheduler = dockerScheduler.SetNatsConnection(ctx, NatsConnection)

	default:
		slog.Error("Invalid executor specified: ", "executor", executorCfg.Executor)
		os.Exit(1)
	}

	router := setupAPIRoute(ctx, kv)
	if err := router.Run(":8080"); err != nil {
		slog.Error("Failed to start API server", "error", err)
	}

	HadesConsumer.DequeueJob(ctx, func(payload payload.QueuePayload) {
		slog.Info("Received job", "id", payload.ID.String())
		slog.Debug("Job payload", "payload", payload)

		if err := scheduler.ScheduleJob(ctx, payload); err != nil {
			slog.Error("Failed to schedule job", "error", err, "id", payload.ID.String())
			return
		}
		slog.Info("Successfully scheduled job", "id", payload.ID.String())
	})
}

func setupAPIRoute(ctx context.Context, kv jetstream.KeyValue) *gin.Engine {
	router := gin.Default()

	router.GET("/api/jobs/:jobId/status", func(c *gin.Context) {
		jobId := c.Param("jobId")

		entry, err := kv.Get(ctx, jobId)
		if err != nil {
			if err == jetstream.ErrKeyNotFound {
				c.JSON(404, gin.H{"error": "Job not found"})
				return
			}
			c.JSON(500, gin.H{"error": "Failed to get job status"})
			return
		}

		c.JSON(200, gin.H{
			"job_id": jobId,
			"status": string(entry.Value()),
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return router
}
