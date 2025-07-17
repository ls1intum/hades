package docker

import (
	"context"
	"log/slog"

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

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
