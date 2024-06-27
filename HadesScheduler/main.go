package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/hibiken/asynq"
	"github.com/ls1intum/hades/hadesScheduler/docker"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"

	"log/slog"
)

var AsynqServer *asynq.Server

type JobScheduler interface {
	ScheduleJob(ctx context.Context, job payload.QueuePayload) error
}

type HadesSchedulerConfig struct {
	Concurrency       uint   `env:"CONCURRENCY" envDefault:"1"`
	FluentdAddr       string `env:"FLUENTD_ADDR" envDefault:""`
	FluentdMaxRetries uint   `env:"FLUENTD_MAX_RETRIES" envDefault:"3"`
	RedisConfig       utils.RedisConfig
}

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

	AsynqServer = utils.SetupQueueServer(cfg.RedisConfig.Addr, cfg.RedisConfig.Pwd, cfg.RedisConfig.TLS_Enabled, int(cfg.Concurrency))

	var scheduler JobScheduler
	switch executorCfg.Executor {
	case "k8s":
		slog.Info("Started HadesScheduler in Kubernetes mode")
		scheduler = k8s.NewK8sScheduler()
	case "docker":
		slog.Info("Started HadesScheduler in Docker mode")
		scheduler = docker.NewDockerScheduler().SetFluentdLogging(cfg.FluentdAddr, cfg.FluentdMaxRetries)
	default:
		slog.Error("Invalid executor specified: ", "executor", executorCfg.Executor)
	}

	AsynqServer.Run(asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		var job payload.QueuePayload
		if err := json.Unmarshal(t.Payload(), &job); err != nil {
			slog.Error("Failed to unmarshal task payload", slog.Any("error", err))
			return err
		}
		slog.Debug("Received job ", "id", job.ID.String())

		if err := scheduler.ScheduleJob(ctx, job); err != nil {
			slog.Error("Failed to schedule job", slog.Any("error", err))
			return err
		}

		return nil
	}))
}
