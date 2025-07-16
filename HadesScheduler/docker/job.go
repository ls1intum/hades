package docker

import (
	"context"
	"log/slog"
	"maps"

	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/payload"
)

type DockerJob struct {
	cli    *client.Client
	logger *slog.Logger
	DockerProps
	payload.QueuePayload
	publisher log.Publisher
}

type jobIDContextKey string

func (d DockerJob) execute(ctx context.Context) error {
	for _, step := range d.Steps {
		d.logger.Info("Executing step", slog.Any("step", step))

		// Copy the global envs and add the step specific ones
		var envs = make(map[string]string)
		maps.Copy(envs, d.Metadata)
		maps.Copy(envs, step.Metadata)
		step.Metadata = envs

		docker_step := DockerStep{
			cli:         d.cli,
			logger:      d.logger,
			DockerProps: d.DockerProps,
			Step:        step,
			publisher:   d.publisher,
		}

		ctx := context.WithValue(ctx, jobIDContextKey("job_id"), d.ID.String())
		err := docker_step.execute(ctx)
		if err != nil {
			d.logger.Error("Failed to execute step", slog.Any("error", err))
			return err
		}
	}
	return nil
}
