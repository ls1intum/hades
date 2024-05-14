package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/hibiken/asynq"
	"github.com/ls1intum/hades/hadesScheduler/docker"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"

	log "github.com/sirupsen/logrus"
)

var AsynqServer *asynq.Server

type JobScheduler interface {
	ScheduleJob(ctx context.Context, job payload.QueuePayload) error
}

type HadesSchedulerConfig struct {
	Concurrency uint `env:"CONCURRENCY" envDefault:"1"`
	RedisConfig utils.RedisConfig
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesSchedulerConfig
	utils.LoadConfig(&cfg)

	var executorCfg utils.ExecutorConfig
	utils.LoadConfig(&executorCfg)
	log.Debug("Executor config: ", executorCfg)

	AsynqServer = utils.SetupQueueServer(cfg.RedisConfig.Addr, cfg.RedisConfig.Pwd, cfg.RedisConfig.TLS_Enabled, int(cfg.Concurrency))

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
