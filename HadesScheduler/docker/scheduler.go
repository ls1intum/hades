package docker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/payload"
)

type Options struct {
	scriptExecutor       string
	containerAutoremove  bool
	cpuLimit             uint
	memoryLimit          string
	volumeName           string
	containerLogsOptions container.LogConfig
}

type Scheduler struct {
	Options
	cli       *client.Client
	publisher log.Publisher
}

func NewScheduler(options ...DockerOption) (*Scheduler, error) {
	scheduler, err := NewDefaultScheduler()
	if err != nil {
		return nil, err
	}

	for _, option := range options {
		if err := option(scheduler); err != nil {
			return nil, err
		}
	}

	return scheduler, nil
}

func NewDefaultScheduler() (*Scheduler, error) {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.WithHost("unix:///var/run/docker.sock"), client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("Failed to create Docker client", slog.Any("error", err))
		return nil, err
	}

	defaultOpts := Options{
		scriptExecutor:      "/bin/bash -c",
		containerAutoremove: false,
		cpuLimit:            0,
		memoryLimit:         "",
	}

	scheduler := &Scheduler{
		cli:       cli,
		Options:   defaultOpts,
		publisher: log.NewNoopPublisher(), // Use no-op publisher by default
	}

	return scheduler, nil
}

func (d Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	var jobLogger *slog.Logger
	var containerLogsOptions container.LogConfig

	jobLogger = slog.Default().With(slog.String("job_id", job.ID.String()))
	containerLogsOptions = container.LogConfig{}

	// Create a unique volume name for this job
	volumeName := fmt.Sprintf("shared-%s", job.ID.String())
	// Create the shared volume
	if err := createSharedVolume(ctx, d.cli, volumeName); err != nil {
		jobLogger.Error("Failed to create shared volume", slog.Any("error", err))
		return err
	}

	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(500 * time.Millisecond)
		if err := deleteSharedVolume(ctx, d.cli, volumeName); err != nil {
			jobLogger.Error("Failed to delete shared volume", slog.Any("error", err))
		}

		jobLogger.Info("Volume deleted", slog.Any("volume", volumeName))
	}()

	// Add created volume to the job's docker config
	jobDockerConfig := d.Options
	jobDockerConfig.volumeName = volumeName
	jobDockerConfig.containerLogsOptions = containerLogsOptions
	dockerJob := Job{
		cli:          d.cli,
		logger:       jobLogger,
		Options:      jobDockerConfig,
		QueuePayload: job,
		publisher:    d.publisher,
	}

	//block to send status first before execution
	if err := d.publisher.PublishJobStatus(ctx, buildlogs.StatusRunning, job.ID.String()); err != nil {
		jobLogger.Warn("failed to publish running status", "error", err)
	}

	err := dockerJob.execute(ctx)
	if err != nil {
		if err := d.publisher.PublishJobStatus(ctx, buildlogs.StatusFailed, job.ID.String()); err != nil {
			jobLogger.Warn("failed to publish failed status", "error", err)
		}
		jobLogger.Error("Failed to execute job", "error", err)
		return err
	}

	if err := d.publisher.PublishJobStatus(ctx, buildlogs.StatusSuccess, job.ID.String()); err != nil {
		jobLogger.Warn("failed to publish success status", "error", err)
	}
	jobLogger.Debug("Job executed successfully", "job_id", job.ID)

	return nil
}
