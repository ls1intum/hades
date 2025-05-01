package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func writeContainerLogsToFile(ctx context.Context, client *client.Client, containerID string, logFilePath string) error {
	out, err := os.Create(logFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	logReader, err := client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	})
	if err != nil {
		return err
	}
	defer logReader.Close()

	_, err = stdcopy.StdCopy(out, out, logReader)
	return err
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
