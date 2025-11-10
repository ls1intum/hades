package docker

import (
	"fmt"

	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/log"
)

type DockerOption func(*Scheduler) error

func WithDockerHost(dockerHost string) DockerOption {
	return func(s *Scheduler) error {
		cli, err := client.NewClientWithOpts(client.WithHost(dockerHost), client.WithAPIVersionNegotiation())
		if err != nil {
			return fmt.Errorf("creating Docker client with host %s: %w", dockerHost, err)
		}
		s.cli = cli
		return nil
	}
}

func WithPublisher(publisher log.Publisher) DockerOption {
	return func(s *Scheduler) error {
		if publisher == nil {
			return fmt.Errorf("nil publisher provided")
		}
		s.publisher = publisher
		return nil
	}
}

func WithScriptExecutor(scriptExecutor string) DockerOption {
	return func(s *Scheduler) error {
		s.scriptExecutor = scriptExecutor
		return nil
	}
}

func WithContainerAutoremove(autoremove bool) DockerOption {
	return func(s *Scheduler) error {
		s.containerAutoremove = autoremove
		return nil
	}
}

func WithCPULimit(cpuLimit uint) DockerOption {
	return func(s *Scheduler) error {
		s.cpuLimit = cpuLimit
		return nil
	}
}

func WithMemoryLimit(memoryLimit string) DockerOption {
	return func(s *Scheduler) error {
		s.memoryLimit = memoryLimit
		return nil
	}
}
