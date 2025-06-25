package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/ls1intum/hades/hadesScheduler/fluentd"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
	slogfluentd "github.com/samber/slog-fluentd/v2"
	"golang.org/x/exp/maps"
)

type DockerEnvConfig struct {
	DockerHost           string `env:"DOCKER_HOST" envDefault:"unix:///var/run/docker.sock"`
	ContainerAutoremove  bool   `env:"DOCKER_CONTAINER_AUTOREMOVE" envDefault:"true"`
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
	cleanupSharedVolumes bool
}

type Scheduler struct {
	cli *client.Client
	DockerProps
	fluentd.FluentdOptions
}

type DockerJob struct {
	cli    *client.Client
	logger *slog.Logger
	DockerProps
	payload.QueuePayload
}

type DockerStep struct {
	cli    *client.Client
	logger *slog.Logger
	DockerProps
	payload.Step
}

type jobIDContextKey string

func NewDockerScheduler() *Scheduler {
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
	}
}

func (d *Scheduler) SetFluentdLogging(addr string, max_retries uint) *Scheduler {
	if addr != "" {
		d.FluentdOptions = fluentd.FluentdOptions{
			Addr:     addr,
			MaxRetry: max_retries,
		}
	}
	return d
}

// Cleanup deletes shared volume after build
func (d *Scheduler) SetCleanupSharedVolumes(cleanup bool) *Scheduler {
	if cleanup {
		slog.Warn("CLEANUP AFTER BUILD ENABLED")
	}

	d.DockerProps.cleanupSharedVolumes = cleanup
	return d
}

func (d Scheduler) ScheduleJob(ctx context.Context, job payload.QueuePayload) error {
	// Create a custom logger only for this job when fluentd is enabled
	var job_logger *slog.Logger
	var container_logs_options container.LogConfig
	if d.FluentdOptions.Addr != "" {
		fluentd_client, err := fluentd.GetFluentdClient(d.FluentdOptions)
		if err != nil {
			slog.Error("Failed to create fluentd client", slog.Any("error", err))
			return err
		}
		job_logger = slog.New(slogfluentd.Option{
			Level:  slog.LevelDebug,
			Client: fluentd_client,
		}.NewFluentdHandler()).With(slog.String("job_id", job.ID.String()))

		// Configure the handler for the container logs
		container_logs_options = container.LogConfig{
			Type: "fluentd",
			Config: map[string]string{
				"fluentd-address":     d.FluentdOptions.Addr,
				"fluentd-max-retries": strconv.FormatUint(uint64(d.FluentdOptions.MaxRetry), 10),
				"labels":              "job_id",
			},
		}
	} else {
		job_logger = slog.Default().With(slog.String("job_id", job.ID.String()))
		container_logs_options = container.LogConfig{}
		job_logger.Warn("No fluentd address provided, using default logger")
	}
	// Create a unique volume name for this job
	volumeName := fmt.Sprintf("shared-%s", job.ID.String())
	// Create the shared volume
	if err := createSharedVolume(ctx, d.cli, volumeName); err != nil {
		job_logger.Error("Failed to create shared volume", slog.Any("error", err))
		return err
	}

	// If cleanup is enabled, delete the shared volume after build
	if d.DockerProps.cleanupSharedVolumes {
		// give Docker a beat to fully unmount
		time.Sleep(1 * time.Second)
		if err := deleteSharedVolume(ctx, d.cli, volumeName); err != nil {
			job_logger.Error("Failed to delete shared volume after build", slog.Any("error", err), slog.String("volume", volumeName))
		} else {
			job_logger.Info("Shared volume deleted after build", slog.String("volume", volumeName))
		}

		msg, err := d.cli.VolumeInspect(ctx, volumeName)
		if err == nil {
			job_logger.Info(msg.Name+" still exists after deletion attempt", slog.Any("volume", msg))
		} else {
			job_logger.Warn("Volume deleted.")
		}
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
	}
	err := docker_job.execute(ctx)
	if err != nil {
		job_logger.Error("Failed to execute job", slog.Any("error", err))
		job_logger.Debug("Failed to execute job", slog.Any("error", err))
		return err
	}

	job_logger.Debug("Job executed successfully", slog.Any("job_id", job.ID))

	// Delete the shared volume after the job is done
	defer func() {
		time.Sleep(1 * time.Second)
		if err := deleteSharedVolume(ctx, d.cli, volumeName); err != nil {
			job_logger.Error("Failed to delete shared volume", slog.Any("error", err))
		}

		job_logger.Info("Volume deleted", slog.Any("volume", volumeName))
	}()

	return nil
}

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

func (s DockerStep) execute(ctx context.Context) error {
	// Pull the images
	err := pullImages(ctx, s.cli, s.Image)
	if err != nil {
		s.logger.Error("Failed to pull image", slog.Any("error", err))
		return err
	}

	var envs []string
	for k, v := range s.Metadata {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	job_id := ctx.Value(jobIDContextKey("job_id")).(string)

	// Add the job_id to the container envs
	envs = append(envs, fmt.Sprintf("UUID=%s", job_id))

	container_config := container.Config{
		Image:      s.Image,
		Env:        envs,
		WorkingDir: "/shared", // Set the working directory to the shared volume
		Labels:     map[string]string{"job_id": job_id},
	}

	host_config := container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: s.volumeName,
				Target: "/shared",
			},
		},
		LogConfig:  s.containerLogsOptions,
		AutoRemove: s.DockerProps.containerAutoremove, // Remove the container after it is done only if the config is set to true
	}

	// Limit the resource usage of the containers
	cpu_limit := utils.FindLimit(int(s.CPULimit), int(s.DockerProps.cpu_limit))
	if cpu_limit != 0 {
		s.logger.Debug("Setting CPU limit to ", "limit", cpu_limit)
		host_config.Resources.NanoCPUs = int64(float64(cpu_limit) * 1e9)
	}
	ram_limit := utils.FindMemoryLimit(s.MemoryLimit, s.DockerProps.memory_limit)
	if ram_limit != 0 {
		s.logger.Debug("Setting RAM limit to ", "limit", ram_limit)
		host_config.Resources.Memory = int64(ram_limit)
	}

	// Create the bash script if there is one
	if s.Script != "" {
		// Overwrite the default entrypoint
		container_config.Entrypoint = strings.Split(s.scriptExecutor, " ")
		container_config.Entrypoint = append(container_config.Entrypoint, s.Script)
	}

	resp, err := s.cli.ContainerCreate(ctx, &container_config, &host_config, nil, nil, "")
	if err != nil {
		s.logger.Error("Failed to create container", slog.Any("error", err))
		return err
	}

	// Start the container
	err = s.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		s.logger.Error("Failed to start container", slog.Any("error", err))
		return err
	}

	// Wait for the container to finish
	statusCh, errCh := s.cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			s.logger.Error("Error waiting for container", slog.Any("error", err), slog.Any("container_id", resp.ID))
			return err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			s.logger.Error("Container exited with status", slog.Any("status", status.StatusCode), slog.Any("container_id", resp.ID), slog.Any("image", s.Image))
			return fmt.Errorf("container exited with status %d", status.StatusCode)
		}
	}

	s.logger.Debug("Container completed", slog.Any("container_id", resp.ID), slog.Any("image", s.Image))
	return nil
}
