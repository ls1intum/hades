package docker

import (
	"context"
	"fmt"
	"github.com/Mtze/HadesCI/hadesScheduler/config"
	"io"
	"sync"
	"time"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	_ "github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"os"
)

var cli *client.Client

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

func (d Scheduler) ScheduleJob(job payload.BuildJob) error {
	ctx := context.Background()

	startOfPull := time.Now()
	// Pull the images
	err := pullImages(ctx, cli, job.BuildConfig.ExecutionContainer, config.CloneContainerImage, config.ResultContainerImage)
	if err != nil {
		log.WithError(err).Error("Failed to pull images")
		return err
	}
	log.Debugf("Pulled images in %s", time.Since(startOfPull))

	startOfVolume := time.Now()
	// Create the shared volume
	err = createSharedVolume(ctx, cli, config.SharedVolumeName)
	if err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}
	log.Debugf("Create Shared Volume in %s", time.Since(startOfVolume))
	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(1 * time.Second)
		startOfDelete := time.Now()
		err = deleteSharedVolume(ctx, cli, config.SharedVolumeName)
		if err != nil {
			log.WithError(err).Error("Failed to delete shared volume")
		}
		log.Debugf("Delete Shared Volume in %s", time.Since(startOfDelete))
	}()

	startOfClone := time.Now()
	// Clone the repository
	err = cloneRepository(ctx, cli, job.Credentials, job.BuildConfig.Repositories...)
	if err != nil {
		log.WithError(err).Error("Failed to clone repository")
		return err
	}
	log.Debugf("Clone repo in %s", time.Since(startOfClone))

	startOfExecute := time.Now()
	err = executeRepository(ctx, cli, job.BuildConfig)
	if err != nil {
		log.WithError(err).Error("Failed to execute repository")
		return err
	}
	log.Debugf("Execute repo in %s", time.Since(startOfExecute))
	log.Debugf("Total time: %s", time.Since(startOfPull))

	return nil
}

func pullImages(ctx context.Context, client *client.Client, images ...string) error {
	var wg sync.WaitGroup
	errorsCh := make(chan error, len(images))

	for _, image := range images {
		wg.Add(1)

		go func(img string) {
			defer wg.Done()

			response, err := client.ImagePull(ctx, img, types.ImagePullOptions{})
			if err != nil {
				errorsCh <- fmt.Errorf("failed to pull image %s: %v", img, err)
				return
			}
			defer response.Close()
			io.Copy(io.Discard, response) // consume the response to prevent potential leaks
		}(image)
	}

	// wait for all goroutines to complete
	wg.Wait()
	close(errorsCh)

	// Collect errors
	var errors []error
	for err := range errorsCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while pulling images: %+v", len(errors), errors)
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
		Image:      config.CloneContainerImage,
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

	statusCh, errCh := client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.WithError(err).Errorf("Error waiting for container with ID %s", resp.ID)
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			log.Errorf("Container with ID %s exited with status %d", resp.ID, status.StatusCode)
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	log.Infof("Container %s started with ID: %s\n", config.CloneContainerImage, resp.ID)
	return nil
}

func executeRepository(ctx context.Context, client *client.Client, buildConfig payload.BuildConfig) error {
	// First, write the Bash script to a temporary file
	scriptPath, err := writeBashScriptToFile("cd /shared", buildConfig.BuildScript)
	if err != nil {
		log.WithError(err).Error("Failed to write bash script to a temporary file")
		return err
	}
	defer os.Remove(scriptPath)

	// Create the container
	resp, err := client.ContainerCreate(ctx, &container.Config{
		Image:      buildConfig.ExecutionContainer,
		Entrypoint: []string{"/bin/sh", "/tmp/script.sh"},
		Volumes: map[string]struct{}{
			"/shared": {},
		},
	}, &defaultHostConfig, nil, nil, "")
	if err != nil {
		return err
	}

	// Copy the script to the container
	err = copyFileToContainer(ctx, client, resp.ID, scriptPath, "/tmp")
	if err != nil {
		log.WithError(err).Error("Failed to copy script to container")
		return err
	}

	// Start the container
	err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// Wait for the container to finish
	statusCh, errCh := client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			log.WithError(err).Errorf("Error waiting for container %s with ID %s", buildConfig.ExecutionContainer, resp.ID)
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			log.Errorf("Container %s with ID %s exited with status %d", buildConfig.ExecutionContainer, resp.ID, status.StatusCode)
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	// Fetch logs and write to a file
	logFilePath := "./logfile.log" // TODO make this configurable
	err = writeContainerLogsToFile(ctx, client, resp.ID, logFilePath)
	if err != nil {
		log.WithError(err).Errorf("Failed to write logs of container %s with ID %s", buildConfig.ExecutionContainer, resp.ID)
		return err
	}

	log.Infof("Container %s with ID: %s completed", buildConfig.ExecutionContainer, resp.ID)
	return nil
}
