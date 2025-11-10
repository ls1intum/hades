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

type Step struct {
	cli    *client.Client
	logger *slog.Logger
	Options
	payload.Step
	publisher log.Publisher
}

func (s Step) execute(ctx context.Context) error {
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

	jobId, ok := ctx.Value(jobIDContextKey("job_id")).(string)
	if !ok {
		return fmt.Errorf("job_id not found in context")
	}

	// Add the job_id to the container envs
	envs = append(envs, fmt.Sprintf("UUID=%s", jobId))

	containerConfig := container.Config{
		Image:      s.Image,
		Env:        envs,
		WorkingDir: "/shared", // Set the working directory to the shared volume
		Labels:     map[string]string{"job_id": jobId},
	}

	hostConfig := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.volumeName,
				Target: "/shared",
			},
		},
		LogConfig:  s.containerLogsOptions,
		AutoRemove: s.Options.containerAutoremove, // Remove the container after it is done only if the config is set to true
	}

	// Limit the resource usage of the containers
	cpuLimit := utils.FindLimit(int(s.CPULimit), int(s.Options.cpuLimit))
	if cpuLimit != 0 {
		s.logger.Debug("Setting CPU limit to ", "limit", cpuLimit)
		hostConfig.Resources.NanoCPUs = int64(float64(cpuLimit) * 1e9)
	}
	ramLimit := utils.FindMemoryLimit(s.MemoryLimit, s.Options.memoryLimit)
	if ramLimit != 0 {
		s.logger.Debug("Setting RAM limit to ", "limit", ramLimit)
		hostConfig.Resources.Memory = ramLimit
	}

	// Create the bash script if there is one
	if s.Script != "" {
		// Overwrite the default entrypoint
		containerConfig.Entrypoint = strings.Split(s.scriptExecutor, " ")
		containerConfig.Entrypoint = append(containerConfig.Entrypoint, s.Script)
	}

	resp, err := s.cli.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, nil, "")
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
	err = processContainerLogs(ctx, s.cli, s.publisher, resp.ID, jobId)

	if err != nil {
		s.logger.Error("Failed to write container logs to NATS", slog.Any("error", err), slog.Any("container_id", resp.ID))
		return err
	} else {
		s.logger.Debug("Container logs written to NATS", slog.Any("container_id", resp.ID), slog.Any("image", s.Image))
	}

	if !s.containerAutoremove {
		if err := removeContainer(ctx, s.cli, resp.ID); err != nil {
			s.logger.Error("Failed to cleanup container", slog.Any("error", err), slog.Any("container_id", resp.ID))
		}
	}

	return nil
}
