package main

import (
	"context"
	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

type DockerScheduler struct{}

func (d *DockerScheduler) ScheduleJob(job payload.BuildJob) error {
	name := job.BuildConfig.ExecutionContainer
	ctx := context.Background()
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer cli.Close()

	// Pull the image
	_, err = cli.ImagePull(ctx, name, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	// Create the container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: name,
		Cmd:   []string{"sleep", "300"},
	}, nil, nil, nil, "")
	if err != nil {
		return err
	}

	// Start the container
	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	log.Info("Container %s started with ID: %s\n", name, resp.ID)
	return nil
}
