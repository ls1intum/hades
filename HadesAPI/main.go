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
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/ls1intum/hades/shared/utils"
)

type HadesAPIConfig struct {
	APIPort    uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	NatsConfig hadesnats.ConnectionConfig
	AuthKey    string `env:"AUTH_KEY"`
}

var cfg HadesAPIConfig

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

	slog.Info("Starting HadesAPI on port", "port", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := setupRouter(cfg.AuthKey, producer)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.APIPort),
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
	slog.Info("HadesAPI shutdown complete")
}
