package main

import (
	"context"
	"os"

	"github.com/ls1intum/hades/hadesScheduler/docker"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"

	"log/slog"
)

var NatsConnection *nats.Conn

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

	var scheduler JobScheduler
	switch executorCfg.Executor {
	case "k8s":
		slog.Info("Started HadesScheduler in Kubernetes mode")
		scheduler = k8s.NewK8sScheduler()
		//TODO: implement scheduler fail handling dito to docker scheduler

	case "docker":
		slog.Info("Started HadesScheduler in Docker mode")

		var dockerCfg docker.EnvConfig
		utils.LoadConfig(&dockerCfg)
		slog.Debug("Docker config", "config", dockerCfg)

		publisher, err := log.NewNATSPublisher(NatsConnection)
		if err != nil {
			slog.Error("Failed to create NATS publisher", "error", err)
		}

		scheduler, err = docker.NewScheduler(
			docker.WithDockerHost(dockerCfg.DockerHost),
			docker.WithScriptExecutor(dockerCfg.DockerScriptExecutor),
			docker.WithContainerAutoremove(dockerCfg.ContainerAutoremove),
			docker.WithCPULimit(dockerCfg.CPULimit),
			docker.WithMemoryLimit(dockerCfg.MemoryLimit),
			docker.WithPublisher(publisher),
		)
		if err != nil {
			slog.Error("Failed to create Docker scheduler", "error", err)
			return
		}
	default:
		slog.Error("Invalid executor specified: ", "executor", executorCfg.Executor)
		os.Exit(1)
	}

	ctx := context.Background()
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
