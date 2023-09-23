package docker

import (
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
)

var defaultHostConfig = container.HostConfig{
	Mounts: []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: sharedVolumeName,
			Target: "/shared",
		},
	},
	AutoRemove: true,
}
