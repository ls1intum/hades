package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/utils"
	"github.com/nats-io/nats.go"
)

const (
	shutdownTimeout    = 30 * time.Second
	natsConnectTimeout = 10 * time.Second
)

// HadesLogManagerConfig holds the configuration for the log manager
type HadesLogManagerConfig struct {
	NatsConfig utils.NatsConfig
	APIPort    string `env:"HADESLOGMANAGER_API_PORT" envDefault:"8081"`
}

func main() {
	// Setup logging
	utils.SetupLogging()

	// Load configuration
	var cfg HadesLogManagerConfig
	utils.LoadConfig(&cfg)

	// Run main application
	if err := run(cfg); err != nil {
		slog.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// run contains the main application logic with proper error handling
func run(cfg HadesLogManagerConfig) error {
	// Connect to NATS server
	nc, err := connectNATS(cfg.NatsConfig)
	if err != nil {
		return err
	}
	defer nc.Close()

	// Create log consumer
	consumer, err := buildlogs.NewHadesLogConsumer(nc)
	if err != nil {
		return err
	}

	// Create context for application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create log aggregator
	var aggregatorConfig AggregatorConfig
	utils.LoadConfig(&aggregatorConfig)
	logAggregator := NewLogAggregator(ctx, consumer, aggregatorConfig)

	// Create dynamic log manager
	dynamicManager := NewDynamicLogManager(nc, consumer, logAggregator)

	// Set up graceful shutdown
	return runWithGracefulShutdown(ctx, cancel, cfg, dynamicManager, logAggregator)
}

// connectNATS establishes connection to NATS server with timeout
func connectNATS(config utils.NatsConfig) (*nats.Conn, error) {
	nc, err := nats.Connect(config.URL, nats.Timeout(natsConnectTimeout))
	if err != nil {
		return nil, err
	}

	slog.Info("Connected to NATS server", "url", config.URL)
	return nc, nil
}

// runWithGracefulShutdown starts services and handles graceful shutdown
func runWithGracefulShutdown(ctx context.Context, cancel context.CancelFunc, cfg HadesLogManagerConfig, dynamicManager LogManager, logAggregator LogAggregator,
) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start the dynamic log manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Starting dynamic log manager")

		if err := dynamicManager.StartListening(ctx); err != nil {
			slog.Error("Dynamic log manager failed", "error", err)
			errChan <- err
		}
	}()

	// Start API server
	router := setupAPIRoute(logAggregator)
	server := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Starting API server", "port", cfg.APIPort)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("API server failed", "error", err)
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	return waitForShutdown(ctx, cancel, server, &wg, errChan)
}

// waitForShutdown waits for OS signal or error and performs graceful shutdown
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server, wg *sync.WaitGroup, errChan chan error,
) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	var shutdownErr error

	select {
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig.String())
	case err := <-errChan:
		slog.Error("Error during operation", "error", err)
		shutdownErr = err
	}

	// Cancel context to stop background goroutines
	slog.Info("Starting graceful shutdown...")
	cancel()

	// Shutdown API server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("API server shutdown error", "error", err)
		if shutdownErr == nil {
			shutdownErr = err
		}
	} else {
		slog.Info("API server shutdown complete")
	}

	// Wait for all goroutines to finish
	wg.Wait()
	slog.Info("Graceful shutdown complete")

	return shutdownErr
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

		// Get job status for specific job
		jobs.GET("/:jobId/status", func(c *gin.Context) {
			jobID := c.Param("jobId")
			status, err := aggregator.GetJobStatus(jobID)

			if err != nil {
				c.JSON(404, gin.H{"error": err.Error()})
				return
			}

			c.JSON(200, gin.H{"status": status})
		})

		// Get active jobs (for testing)
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
