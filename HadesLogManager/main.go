package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesLogManagerConfig
	utils.LoadConfig(&cfg) // Load config from environment

	// Connect to NATS server
	nc, err := nats.Connect(cfg.NatsConfig.URL)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err.Error())
		return
	}
	defer nc.Close()

	slog.Info("Connected to NATS server")

	consumer, err := logs.NewHadesLogConsumer(nc)
	if err != nil {
		slog.Error("Failed to create log consumer", "error", err.Error())
		return
	}

	// Create log aggregator for batching and API access
	var aggregatorConfig AggregatorConfig
	utils.LoadConfig(&aggregatorConfig)
	logAggregator := NewLogAggregator(consumer, aggregatorConfig)

	// Create dynamic log manager
	dynamicManager := NewDynamicLogManager(nc, consumer, logAggregator)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up OS signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// WaitGroup to track background goroutines
	var wg sync.WaitGroup

	// Start the dynamic log manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Starting dynamic log manager")

		if err := dynamicManager.StartListening(ctx); err != nil {
			slog.Error("Dynamic log manager failed", "error", err)
		}
	}()

	router := setupAPIRoute(logAggregator)
	server := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: router,
	}

	// Start API server in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Starting API server", "port", cfg.APIPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("API server failed", "error", err)
		}
	}()

	<-sigChan
	slog.Info("Received shutdown signal, starting graceful shutdown...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("API server shutdown failed", "error", err)
	} else {
		slog.Info("API server shutdown complete")
	}

	wg.Wait()
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
