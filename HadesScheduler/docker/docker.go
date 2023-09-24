package docker

import (
	"context"
	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"os"
)

const (
	cloneContainerImage  = "alpine/git:latest"
	resultContainerImage = "alpine:latest"
	sharedVolumeName     = "shared"
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

func (d *Scheduler) ScheduleJob(job payload.BuildJob) error {
	ctx := context.Background()

	// Pull the images
	err := pullImages(ctx, cli, job.BuildConfig.ExecutionContainer, cloneContainerImage, resultContainerImage)
	if err != nil {
		log.WithError(err).Error("Failed to pull images")
		return err
	}

	// Create the shared volume
	err = createSharedVolume(ctx, cli, sharedVolumeName)
	if err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}

	// Clone the repository
	err = cloneRepository(ctx, cli, job.Credentials, job.BuildConfig.Repositories...)
	if err != nil {
		log.WithError(err).Error("Failed to clone repository")
		return err
	}

	err = executeRepository(ctx, cli, job.BuildConfig)
	if err != nil {
		log.WithError(err).Error("Failed to execute repository")
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

func pullImages(ctx context.Context, client *client.Client, images ...string) error {
	for _, image := range images {
		_, err := client.ImagePull(ctx, image, types.ImagePullOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func cloneRepository(ctx context.Context, client *client.Client, credentials payload.Credentials, repositories ...payload.Repository) error {
	// Use the index to modify the slice in place
	for i := range repositories {
		repositories[i].Path = "/shared" + repositories[i].Path
	}
	commandStr := utils.BuildCloneCommands(credentials, repositories...)
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

func executeRepository(ctx context.Context, client *client.Client, buildConfig payload.BuildConfig) error {
	// First, write the Bash script to a temporary file
	scriptPath, err := writeBashScriptToFile(buildConfig.BuildScript)
	if err != nil {
		log.WithError(err).Error("Failed to write bash script to a temporary file")
		return err
	}
	defer os.Remove(scriptPath)

	hostConfigWithScript := defaultHostConfig
	hostConfigWithScript.Mounts = append(defaultHostConfig.Mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: scriptPath,
		Target: "/tmp/script.sh",
	})
	// Create the container
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image:      buildConfig.ExecutionContainer,
		Entrypoint: []string{"/bin/sh", "/tmp/script.sh"},
		Volumes: map[string]struct{}{
			"/shared":        {},
			"/tmp/script.sh": {}, // this volume will hold our script
		},
	}, &hostConfigWithScript, nil, nil, "")
	if err != nil {
		return err
	}

	// Start the container
	err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	log.Infof("Container %s started with ID: %s\n", buildConfig.ExecutionContainer, resp.ID)
	return nil
}
