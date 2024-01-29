package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/Mtze/HadesCI/hadesScheduler/docker"
	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/hibiken/asynq"

	log "github.com/sirupsen/logrus"
)

var AsynqServer *asynq.Server

type JobScheduler interface {
	ScheduleJob(ctx context.Context, job payload.QueuePayload) error
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg utils.RedisConfig
	utils.LoadConfig(&cfg)

	var executorCfg utils.ExecutorConfig
	utils.LoadConfig(&executorCfg)
	log.Debug("Executor config: ", executorCfg)

	var err error
	AsynqServer = asynq.NewServer(asynq.RedisClientOpt{Addr: cfg.Addr}, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			"critical": 5,
			"high":     4,
			"normal":   3,
			"low":      2,
			"minimal":  1,
		},
		StrictPriority: true,
		Logger:         log.StandardLogger(),
	})
	if AsynqServer == nil {
		log.Panic(err)
	}

	var scheduler JobScheduler

	switch executorCfg.Executor {
	// case "k8s":
	// 	log.Info("Started HadesScheduler in Kubernetes mode")
	// 	kube.Init()
	// 	scheduler = kube.Scheduler{}
	case "docker":
		log.Info("Started HadesScheduler in Docker mode")
		scheduler = docker.NewDockerScheduler()
	default:
		log.Fatalf("Invalid executor specified: %s", executorCfg.Executor)
	}

	AsynqServer.Run(asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		log.Debug("Received task: ", t.Type())
		var job payload.QueuePayload
		if err := json.Unmarshal(t.Payload(), &job); err != nil {
			log.WithError(err).Error("Failed to unmarshal task payload")
			return err
		}

		if err := scheduler.ScheduleJob(ctx, job); err != nil {
			log.WithError(err).Error("Failed to schedule job")
			return err
		}

		return nil
	}))
}
