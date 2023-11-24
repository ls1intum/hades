package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"

	"github.com/Mtze/HadesCI/hadesScheduler/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	log "github.com/sirupsen/logrus"
)

var defaultHostConfig = container.HostConfig{
	Mounts: []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: config.SharedVolumeName,
			Target: "/shared",
		},
	},
	AutoRemove: true,
}

func writeContainerLogsToFile(ctx context.Context, client *client.Client, containerID string, logFilePath string) error {
	out, err := os.Create(logFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	logReader, err := client.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
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

	// Now send the tar archive to CopyToContainer
	err = client.CopyToContainer(ctx, containerID, dstPath, &buf, types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	})
	if err != nil {
		log.WithError(err).Error("Failed to copy script to container")
		return err
	}
	return nil
}
