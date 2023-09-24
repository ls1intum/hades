package docker

import (
	"github.com/Mtze/HadesCI/hadesScheduler/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
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
