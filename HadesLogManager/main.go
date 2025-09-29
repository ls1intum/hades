package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	logs "github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
)

type HadesLogManagerConfig struct {
	NatsConfig utils.NatsConfig
	APIPort    string `env:"API_PORT" envDefault:"8081"`
}

func main() {
	var cfg HadesLogManagerConfig
	utils.LoadConfig(&cfg) // Load config from environment

	// Connect to NATS server
	nc, err := nats.Connect(cfg.NatsConfig.URL)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err.Error())
	}
	defer nc.Close()

	slog.Info("Connected to NATS server")

	consumer, err := logs.NewHadesLogConsumer(nc)
	if err != nil {
		slog.Error("Failed to create log consumer", "error", err.Error())
		return
	}

	// Create log aggregator for batching and API access
	aggregatorConfig := AggregatorConfig{
		BatchSize:     100,
		FlushInterval: 30 * time.Second,
		MaxJobLogs:    1000,
	}
	logAggregator := NewLogAggregator(consumer, aggregatorConfig)

	// Create dynamic log manager
	dynamicManager := NewDynamicLogManager(nc, consumer, logAggregator)

	ctx := context.Background()

	// Start the dynamic log manager
	go func() {
		if err := dynamicManager.StartListening(ctx); err != nil {
			slog.Error("Dynamic log manager failed", "error", err)
		}
	}()

	// Start log aggregation
	go func() {
		if err := logAggregator.StartAggregating(ctx); err != nil {
			slog.Error("Log aggregator failed", "error", err)
		}
	}()

	// Start API server
	router := setupAPIRoute(logAggregator)
	slog.Info("Starting API server", "port", cfg.APIPort)

	if err := http.ListenAndServe(":"+cfg.APIPort, router); err != nil {
		slog.Error("API server failed", "error", err)
	}

}

func setupAPIRoute(aggregator LogAggregator) *gin.Engine {
	r := gin.Default()
	jobs := r.Group("/jobs")
	{
		// Get logs for specific job
		jobs.GET("/:jobId/logs", func(c *gin.Context) {
			jobID := c.Param("jobId")
			logs := aggregator.GetJobLogs(jobID)
			c.JSON(200, gin.H{"logs": logs})
		})

		// Get all active jobs (for testing)
		jobs.GET("", func(c *gin.Context) {
			jobs := aggregator.GetAllJobs()
			c.JSON(200, gin.H{"jobs": jobs})
		})
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
