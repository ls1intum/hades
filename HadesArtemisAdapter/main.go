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
)

const (
	shutdownTimeout = 30 * time.Second
)

// AdapterConfig holds the configuration for the Artemis adapter service.
type AdapterConfig struct {
	APIPort           string `env:"HADESADAPTER_API_PORT" envDefault:"8082"`
	ArtemisBaseURL    string `env:"ARTEMIS_BASE_URL"`
	NewResultEndpoint string `env:"ARTEMIS_NEW_RESULT_ENDPOINT"`
	ArtemisAuthToken  string `env:"ARTEMIS_AUTH_TOKEN"`
}

// main is the application entry point. It initialises logging, loads configuration from environment variables,
// and delegates to run for the core application logic.
func main() {
	// Setup logging
	utils.SetupLogging()

	var cfg AdapterConfig
	utils.LoadConfig(&cfg)

	// Run main application
	if err := run(cfg); err != nil {
		slog.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// run initialises the application context, creates the ArtemisAdapter, and starts the server with graceful shutdown support.
// It returns any fatal error encountered during the lifetime of the application.
func run(cfg AdapterConfig) error {

	// Create context for application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	aa := NewAdapter(ctx, cfg)

	// Set up graceful shutdown
	return runWithGracefulShutdown(ctx, cancel, cfg, aa)
}

// runWithGracefulShutdown configures and starts the HTTP API server in a background goroutine,
// then delegates to waitForShutdown to block until either an OS signal or a fatal server error triggers a graceful teardown.
func runWithGracefulShutdown(ctx context.Context, cancel context.CancelFunc, cfg AdapterConfig, aa *ArtemisAdapter) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start API server
	router := setupAPIRoute(aa)
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

	return waitForShutdown(ctx, cancel, server, &wg, errChan)
}

// waitForShutdown blocks until it receives either an OS signal (SIGINT/SIGTERM) or an error from errChan.
// Once triggered, it cancels the application context, attempts a graceful HTTP server shutdown within shutdownTimeout,
// and waits for all goroutines tracked by wg to complete before returning.
func waitForShutdown(ctx context.Context, cancel context.CancelFunc, server *http.Server, wg *sync.WaitGroup, errChan chan error) error {
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

// setupAPIRoute creates and configures the Gin router with all adapter endpoints.
// It registers the following routes:
//
//   - POST /adapter/logs         — accepts a JSON array of buildlogs.Log, stores it, key: jobID of the first log entry.
//   - POST /adapter/test-results — accepts a JSON ResultDTO, stores it, and forwards it to Artemis using the configured result endpoint.
//   - GET  /health               — returns a simple liveness check response.
func setupAPIRoute(aa *ArtemisAdapter) *gin.Engine {
	r := gin.Default()
	jobs := r.Group("/adapter")
	{
		// Post logs for specific job
		jobs.POST("/logs", func(c *gin.Context) {
			var newLogs []buildlogs.Log
			if err := c.BindJSON(&newLogs); err != nil {
				slog.Error("Failed to bind logs JSON", "error", err)
				return
			}

			if len(newLogs) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "empty logs array"})
				return
			}

			jobID := newLogs[0].JobID
			if err := aa.StoreLogs(jobID, newLogs); err != nil {
				slog.Error("Failed to store logs", "job_id", jobID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store logs"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"status": "stored", "job_id": jobID})
		})

		// Post test results for specific job
		jobs.POST("/test-results", func(c *gin.Context) {
			var newResults ResultDTO
			if err := c.BindJSON(&newResults); err != nil {
				slog.Error("Failed to bind test results JSON", "error", err)
				return
			}

			if err := aa.StoreResults(newResults.UUID, newResults); err != nil {
				slog.Error("Failed to store/send results", "uuid", newResults.UUID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process results"})
				return
			}

			slog.Debug("Stored new test results", "uuid", newResults.UUID)
			c.IndentedJSON(http.StatusCreated, newResults)
		})
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
