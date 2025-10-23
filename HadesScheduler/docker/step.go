package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
)

type DockerStep struct {
	cli    *client.Client
	logger *slog.Logger
	DockerProps
	payload.Step
	publisher log.Publisher
}

func (s DockerStep) execute(ctx context.Context) error {
	// Pull the images
	err := pullImages(ctx, s.cli, s.Image)
	if err != nil {
		s.logger.Error("Failed to pull image", slog.Any("error", err))
		return err
	}

	var envs []string
	for k, v := range s.Metadata {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	job_id, ok := ctx.Value(jobIDContextKey("job_id")).(string)
	if !ok {
		return fmt.Errorf("job_id not found in context")
	}

	// Add the job_id to the container envs
	envs = append(envs, fmt.Sprintf("UUID=%s", job_id))

	container_config := container.Config{
		Image:      s.Image,
		Env:        envs,
		WorkingDir: "/shared", // Set the working directory to the shared volume
		Labels:     map[string]string{"job_id": job_id},
	}

	host_config := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.volumeName,
				Target: "/shared",
			},
		},
		LogConfig: s.containerLogsOptions,
	}

	// Limit the resource usage of the containers
	cpu_limit := utils.FindLimit(int(s.CPULimit), int(s.DockerProps.cpu_limit))
	if cpu_limit != 0 {
		s.logger.Debug("Setting CPU limit to ", "limit", cpu_limit)
		host_config.Resources.NanoCPUs = int64(float64(cpu_limit) * 1e9)
	}
	ram_limit := utils.FindMemoryLimit(s.MemoryLimit, s.DockerProps.memory_limit)
	if ram_limit != 0 {
		s.logger.Debug("Setting RAM limit to ", "limit", ram_limit)
		host_config.Resources.Memory = int64(ram_limit)
	}

	// Create the bash script if there is one
	if s.Script != "" {
		// Overwrite the default entrypoint
		container_config.Entrypoint = strings.Split(s.scriptExecutor, " ")
		container_config.Entrypoint = append(container_config.Entrypoint, s.Script)
	}

	resp, err := s.cli.ContainerCreate(ctx, &container_config, &host_config, nil, nil, "")
	if err != nil {
		s.logger.Error("Failed to create container", slog.Any("error", err))
		return err
	}

	// Start the container
	err = s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		s.logger.Error("Failed to start container", slog.Any("error", err))
		return err
	}

	// Wait for the container to finish
	statusCh, errCh := s.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			s.logger.Error("Error waiting for container", slog.Any("error", err), slog.Any("container_id", resp.ID))
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			s.logger.Error("Container exited with status", slog.Any("status", status.StatusCode), slog.Any("container_id", resp.ID), slog.Any("image", s.Image))
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	s.logger.Debug("Container completed", slog.Any("container_id", resp.ID), slog.Any("image", s.Image))

	// Write the container logs to NATS
	err = processContainerLogs(ctx, s.cli, s.publisher, resp.ID, job_id)

	if err != nil {
		s.logger.Error("Failed to write container logs to NATS", slog.Any("error", err), slog.Any("container_id", resp.ID))
		return err
	} else {
		s.logger.Debug("Container logs written to NATS", slog.Any("container_id", resp.ID), slog.Any("image", s.Image))
	}

	if err := removeContainer(ctx, s.cli, resp.ID); err != nil {
		s.logger.Error("Failed to cleanup container", slog.Any("error", err), slog.Any("container_id", resp.ID))
	}

	return nil
}
