package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	cli                  *client.Client
	script_executor      string
	container_autoremove bool
	cpu_limit            uint
	memory_limit         string
}

type DockerConfig struct {
	DockerHost           string `env:"DOCKER_HOST" envDefault:"unix:///var/run/docker.sock"`
	ContainerAutoremove  bool   `env:"CONTAINER_AUTOREMOVE" envDefault:"true"`
	DockerScriptExecutor string `env:"DOCKER_SCRIPT_EXECUTOR" envDefault:"/bin/bash -c"`
	CPU_limit            uint   `env:"DOCKER_CPU_LIMIT"`    // Number of CPUs - e.g. '6'
	MEMORY_limit         string `env:"DOCKER_MEMORY_LIMIT"` // RAM usage in g or m  - e.g. '4g'
}

func NewDockerScheduler() Scheduler {
	var dockerCfg DockerConfig
	utils.LoadConfig(&dockerCfg)
	log.Debugf("Docker config: %+v", dockerCfg)

	var err error
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.WithHost(dockerCfg.DockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithError(err).Fatal("Failed to create Docker client")
	}
	return Scheduler{
		cli:                  cli,
		container_autoremove: dockerCfg.ContainerAutoremove,
		script_executor:      dockerCfg.DockerScriptExecutor,
		cpu_limit:            dockerCfg.CPU_limit,
		memory_limit:         dockerCfg.MEMORY_limit,
	}
}

func (d Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	// Create a unique volume name for this job
	volumeName := fmt.Sprintf("shared-%d", time.Now().UnixNano())
	// Create the shared volume
	if err := createSharedVolume(ctx, d.cli, volumeName); err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}

	var global_envs []string
	// Read the global env variables from the job metadata
	for k, v := range job.Metadata {
		global_envs = append(global_envs, fmt.Sprintf("%s=%s", k, v))
	}

	for _, step := range job.Steps {
		d.executeStep(ctx, d.cli, step, volumeName, global_envs)
	}

	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(1 * time.Second)
		if err := deleteSharedVolume(ctx, d.cli, volumeName); err != nil {
			log.WithError(err).Error("Failed to delete shared volume")
		}

		global_envs = []string{}
	}()

	return nil
}

func (d Scheduler) executeStep(ctx context.Context, client *client.Client, step payload.Step, volumeName string, envs []string) error {
	// Pull the images
	err := pullImages(ctx, d.cli, step.Image)
	if err != nil {
		log.WithError(err).Errorf("Failed to pull image %s", step.Image)
		return err
	}

	// Copy the global envs and add the step specific ones
	var step_envs []string
	copy(step_envs, envs)
	for k, v := range step.Metadata {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	container_config := container.Config{
		Image:      step.Image,
		Env:        envs,
		WorkingDir: "/shared", // Set the working directory to the shared volume
	}

	host_config := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/shared",
			},
		},
		AutoRemove: d.container_autoremove, // Remove the container after it is done only if the config is set to true
	}

	// Limit the resource usage of the containers
	cpu_limit := utils.FindLimit(int(step.CPULimit), int(d.cpu_limit))
	if cpu_limit != 0 {
		log.Debug("Setting CPU limit to ", cpu_limit)
		host_config.Resources.NanoCPUs = int64(float64(cpu_limit) * 1e9)
	}
	ram_limit := utils.FindMemoryLimit(step.MemoryLimit, d.memory_limit)
	if ram_limit != 0 {
		log.Debug("Setting RAM limit to ", ram_limit)
		host_config.Resources.Memory = int64(ram_limit)
	}

	// Create the bash script if there is one
	if step.Script != "" {
		// Overwrite the default entrypoint
		container_config.Entrypoint = strings.Split(d.script_executor, " ")
		container_config.Entrypoint = append(container_config.Entrypoint, step.Script)
	}

	resp, err := client.ContainerCreate(ctx, &container_config, &host_config, nil, nil, "")
	if err != nil {
		log.WithError(err).Errorf("Failed to create container %s", step.Image)
		return err
	}

	// Start the container
	err = client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		log.WithError(err).Errorf("Failed to start container %s with ID %s", step.Image, resp.ID)
		return err
	}

	// Wait for the container to finish
	statusCh, errCh := client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.WithError(err).Errorf("Error waiting for container %s with ID %s", step.Image, resp.ID)
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			log.Errorf("Container %s with ID %s exited with status %d", step.Image, resp.ID, status.StatusCode)
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	log.Debugf("Container %s with ID: %s completed", step.Image, resp.ID)
	return nil
}
