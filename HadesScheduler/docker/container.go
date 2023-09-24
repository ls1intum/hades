package docker

import (
	"context"
	"github.com/Mtze/HadesCI/hadesScheduler/config"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"os"
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
