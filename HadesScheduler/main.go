package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ls1intum/hades/hadesScheduler/docker"
	"github.com/ls1intum/hades/hadesScheduler/k8s"
	"github.com/ls1intum/hades/hadesScheduler/log"
	hades "github.com/ls1intum/hades/shared"
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
)

type HadesSchedulerConfig struct {
	Concurrency uint `env:"CONCURRENCY" envDefault:"1"`
	NatsConfig  hadesnats.ConnectionConfig
}

func main() {
	if isDebug := os.Getenv("DEBUG"); isDebug == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesSchedulerConfig
	utils.LoadConfig(&cfg)

	var executorCfg utils.ExecutorConfig
	utils.LoadConfig(&executorCfg)
	slog.Debug("Executor config: ", "config", executorCfg)

	natsConn, err := hadesnats.SetupDefaultNatsConnection(cfg.NatsConfig)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsConn.Close()

	consumer, err := hadesnats.NewHadesConsumer(natsConn, cfg.Concurrency)
	if err != nil {
		slog.Error("Failed to create Hades consumer", "error", err)
		os.Exit(1)
	}

	var scheduler hades.JobScheduler
	switch executorCfg.Executor {
	case "k8s":
		slog.Info("Started HadesScheduler in Kubernetes mode")
		k8sScheduler, err := k8s.NewK8sScheduler(natsConn)
		if err != nil {
			slog.Error("Failed to create k8s scheduler", "error", err)
			os.Exit(1)
		}
		scheduler = k8sScheduler

	case "docker":
		slog.Info("Started HadesScheduler in Docker mode")

		var dockerCfg docker.EnvConfig
		utils.LoadConfig(&dockerCfg)
		slog.Debug("Docker config", "config", dockerCfg)

		publisher, err := log.NewNATSPublisher(natsConn)
		if err != nil {
			slog.Error("Failed to create NATS publisher", "error", err)
			os.Exit(1)
		}

		scheduler, err = docker.NewScheduler(
			docker.WithDockerHost(dockerCfg.DockerHost),
			docker.WithScriptExecutor(dockerCfg.DockerScriptExecutor),
			docker.WithContainerAutoremove(dockerCfg.ContainerAutoremove),
			docker.WithCPULimit(dockerCfg.CPULimit),
			docker.WithMemoryLimit(dockerCfg.MemoryLimit),
			docker.WithLogPublisher(publisher),
			docker.WithStatusPublisher(publisher),
		)
		if err != nil {
			slog.Error("Failed to create Docker scheduler", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("Invalid executor specified: ", "executor", executorCfg.Executor)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig.String())
		cancel()
	}()

	consumer.DequeueJob(ctx, func(p payload.QueuePayload) {
		slog.Info("Received job", "id", p.ID.String())
		slog.Debug("Job payload", "payload", p)

		if err := scheduler.ScheduleJob(ctx, p); err != nil {
			slog.Error("Failed to schedule job", "error", err, "id", p.ID.String())
			return
		}
		slog.Info("Successfully scheduled job", "id", p.ID.String())
	})

	slog.Info("Scheduler shutdown complete")
}
