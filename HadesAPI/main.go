package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ls1intum/hades/hadesAPI/dashboard"
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/ls1intum/hades/shared/utils"
)

// HadesAPIConfig holds the API server configuration. An empty AuthKey disables
// Basic Auth on the /build endpoint.
type HadesAPIConfig struct {
	APIPort    uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	NatsConfig hadesnats.ConnectionConfig
	AuthKey    string `env:"AUTH_KEY"`
}

var cfg HadesAPIConfig

// @title                      HadesAPI
// @version                    1.0
// @description                Job-submission API for Hades, a scalable scheduler for containerized jobs. Submit a multi-step job and it is validated, assigned a UUID, and queued on NATS by priority for the scheduler to execute.
// @BasePath                   /
// @securityDefinitions.basic  BasicAuth
func main() {
	utils.SetupLogging()

	if err := utils.LoadConfig(&cfg); err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("HadesAPI configuration",
		"api_port", cfg.APIPort,
		"nats_url", cfg.NatsConfig.URL,
		"nats_tls", cfg.NatsConfig.TLS,
		"auth_enabled", cfg.AuthKey != "",
	)

	natsConn, err := hadesnats.SetupDefaultNatsConnection(cfg.NatsConfig)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		os.Exit(1)
	}
	defer natsConn.Close()

	producer, err := hadesnats.NewHadesPublisher(natsConn)
	if err != nil {
		slog.Error("Failed to create HadesProducer", "error", err)
		os.Exit(1)
	}

	// Dashboard lifecycle context, cancelled on shutdown to stop its NATS
	// subscription and background loops.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dashCfg, err := dashboard.LoadConfig()
	if err != nil {
		slog.Error("Failed to load dashboard configuration", "error", err)
		os.Exit(1)
	}
	// The dashboard is always constructed so its /api routes can return a clear
	// 503 when it is not configured; it only subscribes to NATS and serves the
	// SPA once credentials are present.
	dash, err := dashboard.NewServer(ctx, dashCfg, natsConn)
	if err != nil {
		slog.Error("Failed to create dashboard", "error", err)
		os.Exit(1)
	}
	if dash.Enabled() {
		if err := dash.Start(ctx); err != nil {
			slog.Error("Failed to start dashboard", "error", err)
			os.Exit(1)
		}
		slog.Info("Dashboard enabled")
	} else {
		slog.Warn("Dashboard disabled (set DASHBOARD_USERNAME, DASHBOARD_PASSWORD_HASH and DASHBOARD_SESSION_SECRET to enable)")
	}

	slog.Info("Starting HadesAPI on port", "port", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := setupRouter(cfg.AuthKey, producer, producer, dash)

	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", cfg.APIPort),
		Handler:     r,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout is intentionally disabled: the dashboard SSE endpoint
		// (/api/stream) is a long-lived response that a non-zero server-wide
		// WriteTimeout would sever. Reads are still bounded by ReadTimeout and
		// ReadHeaderTimeout.
		WriteTimeout:      0,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to start HadesAPI", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	slog.Info("Received shutdown signal", "signal", sig.String())

	// Stop the dashboard's background work before draining HTTP.
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
	slog.Info("HadesAPI shutdown complete")
}
