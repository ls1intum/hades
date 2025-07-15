package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/ls1intum/hades/hadesScheduler/log"
)

func processContainerLogs(ctx context.Context, client *client.Client, publisher log.Publisher, containerID, jobID string) error {
	stdout, stderr, err := getContainerLogs(ctx, client, containerID)
	if err != nil {
		return fmt.Errorf("getting container logs: %w", err)
	}

	buildJobLog, err := log.ParseContainerLogs(stdout, stderr, containerID)
	if err != nil {
		return fmt.Errorf("parsing container logs: %w", err)
	}

	buildJobLog.JobID = jobID
	slog.Debug("Pased container logs was of", slog.String("jobID", jobID))

	return publisher.PublishLogs(buildJobLog)
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

func (s *DockerStep) removeContainer(ctx context.Context, containerID string) error {
	if err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true, // Kill if running, then remove
		RemoveVolumes: true, // Clean up any volumes
	}); err != nil {
		return fmt.Errorf("failed to cleanup container %s: %w", containerID, err)
	}

	slog.Info("Container cleanup done", slog.String("container_id", containerID))
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
	// Read the script content
	scriptContent, err := io.ReadAll(scriptFile)
	if err != nil {
		return err
	}
	defer tw.Close()

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
