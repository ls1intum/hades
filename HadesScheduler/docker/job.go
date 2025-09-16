package docker

import (
	"context"
	"log/slog"
	"maps"

	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	"github.com/ls1intum/hades/hadesScheduler/log"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/nats-io/nats.go/jetstream"
)

type DockerJob struct {
	cli    *client.Client
	logger *slog.Logger
	DockerProps
	payload.QueuePayload
	publisher log.NATSPublisher
	status    BuildStatus
	kv        jetstream.KeyValue
}

type BuildStatus string

const (
	BuildStatusExecuting BuildStatus = "executing"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusFinished  BuildStatus = "finished"
)

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

		stepCtx := context.WithValue(ctx, jobIDContextKey("job_id"), d.ID.String())
		err := docker_step.execute(stepCtx)
		if err != nil {
			d.logger.Error("Failed to execute step", slog.Any("error", err))
			return err
		}
	}
	return nil
}

func (d *DockerJob) SetStatus(ctx context.Context, status BuildStatus) {
	d.status = status
	// Update both NATS and publish event
	d.kv.Put(ctx, d.ID.String(), []byte(status))
	d.publisher.PublishJobStatus(string(status), d.ID.String())
}

func (d *DockerJob) GetStatus() BuildStatus {
	return d.status
}

func setupAPIRoute(d DockerJob) *gin.Engine {
	router := gin.Default()

	// Get status for specific job
	router.GET("/api/jobs/:jobId/status", func(c *gin.Context) {
		jobId := c.Param("jobId")

		// Validate that the requested jobId matches this DockerJob
		if jobId != d.ID.String() {
			c.JSON(404, gin.H{"error": "Job not found"})
			return
		}

		status := d.GetStatus()
		c.JSON(200, gin.H{"status": status})
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return router
}
