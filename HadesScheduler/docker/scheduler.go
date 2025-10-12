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
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
)

type DockerEnvConfig struct {
	DockerHost           string `env:"DOCKER_HOST" envDefault:"unix:///var/run/docker.sock"`
	ContainerAutoremove  bool   `env:"DOCKER_CONTAINER_AUTOREMOVE" envDefault:"false"`
	DockerScriptExecutor string `env:"DOCKER_SCRIPT_EXECUTOR" envDefault:"/bin/bash -c"`
	CPU_limit            uint   `env:"DOCKER_CPU_LIMIT"`    // Number of CPUs - e.g. '6'
	MEMORY_limit         string `env:"DOCKER_MEMORY_LIMIT"` // RAM usage in g or m  - e.g. '4g'
}

type DockerProps struct {
	scriptExecutor       string
	containerAutoremove  bool
	cpu_limit            uint
	memory_limit         string
	volumeName           string
	containerLogsOptions container.LogConfig
}

type Scheduler struct {
	cli *client.Client
	DockerProps
	publisher log.Publisher
}

func NewDockerScheduler() (*Scheduler, error) {
	var dockerCfg DockerEnvConfig
	utils.LoadConfig(&dockerCfg)
	slog.Debug("Docker config", "config", dockerCfg)

	var err error
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.WithHost(dockerCfg.DockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("Failed to create Docker client", slog.Any("error", err))
	}
	return &Scheduler{
		cli: cli,
		DockerProps: DockerProps{
			scriptExecutor:      dockerCfg.DockerScriptExecutor,
			containerAutoremove: dockerCfg.ContainerAutoremove,
			cpu_limit:           dockerCfg.CPU_limit,
			memory_limit:        dockerCfg.MEMORY_limit,
		},
	}, nil
}

func (d *Scheduler) SetNatsConnection(nc *nats.Conn) *Scheduler {
	if nc != nil {
		publisher, err := log.NewNATSPublisher(nc)
		if err != nil {
			slog.Error("Failed to create NATS publisher", slog.Any("error", err))
		} else {
			d.publisher = *publisher
		}
	} else {
		slog.Warn("NATS connection is nil, logs nor status will be published")
	}
	return d
}

func (d Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	var job_logger *slog.Logger
	var container_logs_options container.LogConfig

	job_logger = slog.Default().With(slog.String("job_id", job.ID.String()))
	container_logs_options = container.LogConfig{}

	// Create a unique volume name for this job
	volumeName := fmt.Sprintf("shared-%s", job.ID.String())
	// Create the shared volume
	if err := createSharedVolume(ctx, d.cli, volumeName); err != nil {
		job_logger.Error("Failed to create shared volume", slog.Any("error", err))
		return err
	}

	// Add created volume to the job's docker config
	jobDockerConfig := d.DockerProps
	jobDockerConfig.volumeName = volumeName
	jobDockerConfig.containerLogsOptions = container_logs_options
	docker_job := DockerJob{
		cli:          d.cli,
		logger:       job_logger,
		DockerProps:  jobDockerConfig,
		QueuePayload: job,
		publisher:    d.publisher,
	}

	//block to send status first before execution
	if err := d.publisher.PublishJobStatus(buildlogs.StatusRunning, job.ID.String()); err != nil {
		job_logger.Warn("failed to publish success status", slog.Any("error", err))
	}

	err := docker_job.execute(ctx)
	if err != nil {
		if err := d.publisher.PublishJobStatus(buildlogs.StatusFailed, job.ID.String()); err != nil {
			job_logger.Warn("failed to publish success status", slog.Any("error", err))
		}
		job_logger.Error("Failed to execute job", slog.Any("error", err))
		return err
	}

	if err := d.publisher.PublishJobStatus(buildlogs.StatusSuccess, job.ID.String()); err != nil {
		job_logger.Warn("failed to publish success status", slog.Any("error", err))
	}
	job_logger.Debug("Job executed successfully", slog.Any("job_id", job.ID))

	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(500 * time.Millisecond)
		if err := deleteSharedVolume(ctx, d.cli, volumeName); err != nil {
			job_logger.Error("Failed to delete shared volume", slog.Any("error", err))
		}

		job_logger.Info("Volume deleted", slog.Any("volume", volumeName))
	}()

	return nil
}
