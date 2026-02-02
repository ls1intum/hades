package docker

import (
	"context"
	"log/slog"
	"maps"

	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/payload"
)

type Job struct {
	cli    *client.Client
	logger *slog.Logger
	Options
	payload.QueuePayload
	publisher buildlogs.LogPublisher
}

type jobIDContextKey string

func (d Job) execute(ctx context.Context) error {
	for _, step := range d.Steps {
		d.logger.Info("Executing step", slog.Any("step", step))

		// Copy the global envs and add the step specific ones
		var envs = make(map[string]string)
		maps.Copy(envs, d.Metadata)
		maps.Copy(envs, step.Metadata)
		envs["UUID"] = d.ID.String()
		step.Metadata = envs

		dockerStep := Step{
			cli:       d.cli,
			logger:    d.logger,
			Options:   d.Options,
			Step:      step,
			publisher: d.publisher,
		}

		stepCtx := context.WithValue(ctx, jobIDContextKey("job_id"), d.ID.String())
		err := dockerStep.execute(stepCtx)
		if err != nil {
			d.logger.Error("Failed to execute step", slog.Any("error", err))
			return err
		}
	}
	return nil
}
