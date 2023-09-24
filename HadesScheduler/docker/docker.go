package docker

import (
	"context"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

const (
	cloneContainerImage = "alpine/git:latest"
	sharedVolumeName    = "shared"
)

var cli *client.Client

type Scheduler struct{}

func init() {
	var err error
	// Create a new Docker client
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Docker client")
	}
}

func (d Scheduler) ScheduleJob(job payload.BuildJob) error {
	ctx := context.Background()

	// Create the shared volume
	err := createSharedVolume(ctx, cli, sharedVolumeName)
	if err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}

	// Clone the repository
	err = cloneRepository(ctx, cli, job.BuildConfig.Repositories...)
	if err != nil {
		log.WithError(err).Error("Failed to clone repository")
		return err
	}

	// TODO enable deletion of shared volume
	//time.Sleep(5 * time.Second)
	//err = deleteSharedVolume(ctx, cli, sharedVolumeName)
	//if err != nil {
	//	log.WithError(err).Error("Failed to delete shared volume")
	//	return err
	//}

	return nil
}

func cloneRepository(ctx context.Context, client *client.Client, repositories ...payload.Repository) error {
	// Pull the image
	_, err := client.ImagePull(ctx, cloneContainerImage, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	// Use the index to modify the slice in place
	for i := range repositories {
		repositories[i].Path = "/shared" + repositories[i].Path
	}
	commandStr := utils.BuildCloneCommands(repositories...)
	log.Debug(commandStr)

	// Create the container
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image:      cloneContainerImage,
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd:        []string{commandStr},
		Volumes: map[string]struct{}{
			"/shared": {},
		},
	}, &defaultHostConfig, nil, nil, "")
	if err != nil {
		return err
	}

	// Start the container
	err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	log.Infof("Container %s started with ID: %s\n", cloneContainerImage, resp.ID)
	return nil
}
