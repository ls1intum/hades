package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/docker/docker/api/types/container"
	image_types "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/buildlogs"
)

func processContainerLogs(ctx context.Context, client *client.Client, publisher buildlogs.LogPublisher, containerID, jobID string) error {
	stdout, stderr, err := getContainerLogs(ctx, client, containerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	parser := log.NewStdLogParser(stdout, stderr)
	buildJobLog, err := parser.ParseContainerLogs(containerID, jobID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	slog.Debug("Parsed container logs", "job_id", jobID, "container_id", containerID)
	return publisher.PublishLog(ctx, buildJobLog)
}

// retrieves and demultiplexes container logs
func getContainerLogs(ctx context.Context, client *client.Client, containerID string) (*bytes.Buffer, *bytes.Buffer, error) {
	logReader, err := client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("getting container logs: %w", err)
	}
	defer logReader.Close()

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	if _, err := stdcopy.StdCopy(stdout, stderr, logReader); err != nil {
		return nil, nil, fmt.Errorf("demultiplexing logs: %w", err)
	}

	return stdout, stderr, nil
}

func removeContainer(ctx context.Context, client *client.Client, containerID string) error {
	if err := client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true, // Kill if running, then remove
		RemoveVolumes: true, // Clean up any volumes
	}); err != nil {
		return fmt.Errorf("failed to cleanup container %s: %w", containerID, err)
	}

	slog.Info("Container cleanup done", slog.String("container_id", containerID))
	return nil
}

func pullImages(ctx context.Context, client *client.Client, images ...string) error {
	var wg sync.WaitGroup
	errorsCh := make(chan error, len(images))

	for _, image := range images {
		wg.Add(1)

		go func(img string) {
			defer wg.Done()

			response, err := client.ImagePull(ctx, img, image_types.PullOptions{})
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

func copyFileToContainer(ctx context.Context, client *client.Client, containerID, srcPath, dstPath string) error {
	scriptFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer scriptFile.Close()

	// Create a buffer to hold the tar archive
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	// Read the script content
	scriptContent, err := io.ReadAll(scriptFile)
	if err != nil {
		return err
	}

	// Add the script content to the tar archive
	tarHeader := &tar.Header{
		Name: "script.sh",
		Size: int64(len(scriptContent)),
		Mode: 0755, // Make sure the script is executable
	}
	if err := tw.WriteHeader(tarHeader); err != nil {
		return err
	}
	if _, err := tw.Write(scriptContent); err != nil {
		return err
	}

	opts := container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
		// CopyUIDGID: true,
	}

	err = client.CopyToContainer(ctx, containerID, dstPath, &buf, opts)
	if err != nil {
		slog.Error("Failed to copy script to container", slog.Any("error", err))
		return err
	}
	return nil
}

func createSharedVolume(ctx context.Context, client *client.Client, name string) error {
	// Create the volume
	_, err := client.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	if err != nil {
		slog.Error("Failed to create shared volume", slog.Any("error", err))
		return err
	}

	slog.Debug("Volume created", slog.Any("volume", name))
	return nil
}

func deleteSharedVolume(ctx context.Context, client *client.Client, name string) error {
	// Delete the volume
	err := client.VolumeRemove(ctx, name, true)
	if err != nil {
		slog.Error("Failed to delete shared volume", slog.Any("error", err))
		return err
	}

	slog.Debug("Volume deleted", slog.Any("volume", name))

	return nil
}
