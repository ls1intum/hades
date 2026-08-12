package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

func processContainerLogs(ctx context.Context, cli *client.Client, publisher buildlogs.LogPublisher, containerID, jobID string) error {
	stdout, stderr, err := getContainerLogs(ctx, cli, containerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	parser := log.NewStdLogParser(stdout, stderr)
	buildJobLog, err := parser.ParseContainerLogs(containerID, jobID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	slog.Debug("Parsed container logs", "job_id", jobID, "container_id", containerID)
	return publisher.PublishJobLog(ctx, buildJobLog)
}

// retrieves and demultiplexes container logs
func getContainerLogs(ctx context.Context, cli *client.Client, containerID string) (*bytes.Buffer, *bytes.Buffer, error) {
	logReader, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
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

func removeContainer(ctx context.Context, cli *client.Client, containerID string) error {
	if _, err := cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force:         true, // Kill if running, then remove
		RemoveVolumes: true, // Clean up any volumes
	}); err != nil {
		return fmt.Errorf("failed to cleanup container %s: %w", containerID, err)
	}

	slog.Info("Container cleanup done", slog.String("container_id", containerID))
	return nil
}

func pullImages(ctx context.Context, cli *client.Client, images ...string) error {
	var wg sync.WaitGroup
	errorsCh := make(chan error, len(images))

	for _, image := range images {
		wg.Add(1)

		go func(img string) {
			defer wg.Done()

			response, err := cli.ImagePull(ctx, img, client.ImagePullOptions{})
			if err != nil {
				errorsCh <- fmt.Errorf("failed to pull image %s: %w", img, err)
				return
			}
			defer response.Close()
			// Decode the pull response stream to completion so that errors the
			// daemon reports in-band (auth failures, missing manifests, ...) are
			// surfaced - not just transport errors returned by ImagePull itself.
			// A plain io.Copy would drain the bytes but ignore those JSON records.
			if err := jsonmessage.DisplayJSONMessagesStream(response, io.Discard, 0, false, nil); err != nil {
				errorsCh <- fmt.Errorf("failed to pull image %s: %w", img, err)
			}
		}(image)
	}

	// wait for all goroutines to complete
	wg.Wait()
	close(errorsCh)

	// Collect errors, keeping them wrapped so callers can still use errors.Is/As.
	var pullErrs []error
	for err := range errorsCh {
		pullErrs = append(pullErrs, err)
	}

	if len(pullErrs) > 0 {
		return fmt.Errorf("encountered %d errors while pulling images: %w", len(pullErrs), errors.Join(pullErrs...))
	}

	return nil
}

func createSharedVolume(ctx context.Context, cli *client.Client, name string) error {
	// Create the volume
	_, err := cli.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name: name,
	})
	if err != nil {
		slog.Error("Failed to create shared volume", slog.Any("error", err))
		return err
	}

	slog.Debug("Volume created", slog.Any("volume", name))
	return nil
}

func deleteSharedVolume(ctx context.Context, cli *client.Client, name string) error {
	// Delete the volume
	_, err := cli.VolumeRemove(ctx, name, client.VolumeRemoveOptions{Force: true})
	if err != nil {
		slog.Error("Failed to delete shared volume", slog.Any("error", err))
		return err
	}

	slog.Debug("Volume deleted", slog.Any("volume", name))

	return nil
}
