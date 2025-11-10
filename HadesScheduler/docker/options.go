package docker

import (
	"log/slog"

	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/log"
)

type DockerOption func(*Scheduler)

func WithDockerHost(dockerHost string) DockerOption {
	return func(s *Scheduler) {
		cli, err := client.NewClientWithOpts(client.WithHost(dockerHost), client.WithAPIVersionNegotiation())
		if err != nil {
			slog.Error("Failed to create Docker client", slog.Any("error", err))
			return
		}
		s.cli = cli
	}
}

func WithPublisher(publisher log.Publisher) DockerOption {
	return func(s *Scheduler) {
		s.publisher = publisher
	}
}

func WithScriptExecutor(scriptExecutor string) DockerOption {
	return func(s *Scheduler) {
		s.scriptExecutor = scriptExecutor
	}
}

func WithContainerAutoremove(autoremove bool) DockerOption {
	return func(s *Scheduler) {
		s.containerAutoremove = autoremove
	}
}

func WithCPULimit(cpuLimit uint) DockerOption {
	return func(s *Scheduler) {
		s.cpuLimit = cpuLimit
	}
}

func WithMemoryLimit(memoryLimit string) DockerOption {
	return func(s *Scheduler) {
		s.memoryLimit = memoryLimit
	}
}
