package docker

import (
	"context"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
)

func createSharedVolume(ctx context.Context, client *client.Client, name string) error {
	// Create the volume
	_, err := client.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	if err != nil {
		log.WithError(err).Error("Failed to create shared volume")
		return err
	}

	log.Debugf("Volume %s created", name)
	return nil
}

func deleteSharedVolume(ctx context.Context, client *client.Client, name string) error {
	// Delete the volume
	err := client.VolumeRemove(ctx, name, true)
	if err != nil {
		log.WithError(err).Error("Failed to delete shared volume")
		return err
	}

	return nil
}
