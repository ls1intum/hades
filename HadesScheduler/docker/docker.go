package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

var cli *client.Client
var global_envs []string = []string{}

type Scheduler struct{}

type DockerConfig struct {
	DockerHost string `env:"DOCKER_HOST" envDefault:"unix:///var/run/docker.sock"`
}

func init() {
	var DockerCfg DockerConfig
	utils.LoadConfig(&DockerCfg)

	var err error
	// Create a new Docker client
	cli, err = client.NewClientWithOpts(client.WithHost(DockerCfg.DockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		log.WithError(err).Fatal("Failed to create Docker client")
	}
}

func (d Scheduler) ScheduleJob(job payload.QueuePayload) error {
	ctx := context.Background()

	// Create a unique volume name for this job
	volumeName := fmt.Sprintf("shared-%d", time.Now().UnixNano())
	// Create the shared volume
	if err := createSharedVolume(ctx, cli, volumeName); err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}

	// Read the global env variables from the job metadata
	for k, v := range job.Metadata {
		global_envs = append(global_envs, fmt.Sprintf("%s=%s", k, v))
	}

	for _, step := range job.Steps {
		executeStep(ctx, cli, step, volumeName)
	}

	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(1 * time.Second)
		if err := deleteSharedVolume(ctx, cli, volumeName); err != nil {
			log.WithError(err).Error("Failed to delete shared volume")
		}

		global_envs = []string{}
	}()

	return nil
}

func executeStep(ctx context.Context, client *client.Client, step payload.Step, volumeName string) error {
	// Pull the images
	err := pullImages(ctx, cli, step.Image)
	if err != nil {
		log.WithError(err).Errorf("Failed to pull image %s", step.Image)
		return err
	}

	// Copy the global envs and add the step specific ones
	var envs []string
	copy(envs, global_envs)
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
		AutoRemove: true,
	}

	// Create the bash script if there is one
	if step.Script != "" {
		// Overwrite the default entrypoint
		container_config.Entrypoint = []string{"/bin/bash", "-c", step.Script}
	}

	resp, err := client.ContainerCreate(ctx, &container_config, &host_config, nil, nil, "")
	if err != nil {
		log.WithError(err).Errorf("Failed to create container %s", step.Image)
		return err
	}

	// Start the container
	err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
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
