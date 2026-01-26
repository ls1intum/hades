package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joshdk/go-junit"
	"github.com/ls1intum/hades/shared/buildlogs"
	"github.com/ls1intum/hades/shared/utils"
)

type HadesArtemisAdapterConfig struct {
	APIPort string `env:"API_PORT" envDefault:"8082"`
}

// ResultMetadata populated from environment variables
type ResultMetadata struct {
	JobName                  string `json:"jobName" env:"JOB_NAME"`
	UUID                     string `json:"uuid" env:"UUID"`
	AssignmentRepoBranchName string `json:"assignmentRepoBranchName" env:"ASSIGNMENT_REPO_BRANCH_NAME" envDefault:"main"`
	IsBuildSuccessful        bool   `json:"isBuildSuccessful" env:"IS_BUILD_SUCCESSFUL"`
	AssignmentRepoCommitHash string `json:"assignmentRepoCommitHash" env:"ASSIGNMENT_REPO_COMMIT_HASH"`
	TestsRepoCommitHash      string `json:"testsRepoCommitHash" env:"TESTS_REPO_COMMIT_HASH"`
	BuildCompletionTime      string `json:"buildCompletionTime" env:"BUILD_COMPLETION_TIME"`
}
type ResultDTO struct {
	ResultMetadata
	BuildJobs []junit.Suite `json:"buildJobs"`
}

func main() {
	// Setup logging
	utils.SetupLogging()

	// Load configuration
	var cfg HadesArtemisAdapterConfig
	utils.LoadConfig(&cfg)

	// Run main application
	if err := run(cfg); err != nil {
		slog.Error("Application error", "error", err)
		os.Exit(1)
	}
}

// run contains the main application logic with proper error handling
func run(cfg HadesArtemisAdapterConfig) error {

	// Create context for application lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up graceful shutdown
	return runWithGracefulShutdown(ctx, cancel, cfg)
}

// runWithGracefulShutdown starts services and handles graceful shutdown
func runWithGracefulShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	cfg HadesArtemisAdapterConfig,
) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	// Start API server
	router := setupAPIRoute()
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
func waitForShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	server *http.Server,
	wg *sync.WaitGroup,
	errChan chan error,
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

func setupAPIRoute() *gin.Engine {
	r := gin.Default()
	jobs := r.Group("/adapter")
	{
		// post logs for specific job
		jobs.POST("/logs", func(c *gin.Context) {
			var newLogs buildlogs.Log
			if err := c.BindJSON(&newLogs); err != nil {
				return
			}

			// logs = append(logs, newLogs)
			c.IndentedJSON(http.StatusCreated, newLogs)
		})

		// post results for specific job
		jobs.POST("/results", func(c *gin.Context) {
			var newResults ResultDTO
			if err := c.BindJSON(&newResults); err != nil {
				return
			}

			// Pretty print the whole thing
			jsonBytes, _ := json.MarshalIndent(newResults, "", "  ")
			fmt.Println(string(jsonBytes))

			// results = append(results, newResults)
			c.IndentedJSON(http.StatusCreated, newResults)
		})
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
