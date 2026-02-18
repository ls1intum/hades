package main

import (
	"fmt"
	"os"

	"log/slog"

	"github.com/gin-gonic/gin"
	hades "github.com/ls1intum/hades/shared"
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/ls1intum/hades/shared/utils"
)

type HadesAPIConfig struct {
	APIPort           uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	NatsConfig        hadesnats.ConnectionConfig
	AuthKey           string `env:"AUTH_KEY"`
	PrometheusAddress string `env:"PROMETHEUS_ADDRESS" envDefault:""`
	// How long the task should be kept for monitoring
	RetentionTime uint `env:"RETENTION_IN_MIN" envDefault:"30"`
	MaxRetries    uint `env:"MAX_RETRIES" envDefault:"3"`
	Timeout       uint `env:"TIMEOUT_IN_MIN" envDefault:"0"`
}

var cfg HadesAPIConfig

var HadesProducer hades.JobPublisher

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Warn("DEBUG MODE ENABLED")
	}

	utils.LoadConfig(&cfg)

	var err error
	NatsConnection, err := hadesnats.SetupDefaultNatsConnection(cfg.NatsConfig)
	if err != nil {
		slog.Error("Failed to connect to NATS", "error", err)
		return
	}
	defer NatsConnection.Close()

	HadesProducer, err = hadesnats.NewHadesPublisher(NatsConnection)
	if err != nil {
		slog.Error("Failed to create HadesProducer", "error", err)
		return
	}

	slog.Info("Starting HadesAPI on port", "port", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := setupRouter(cfg.AuthKey)

	slog.Error("Failed to start HadesAPI", "error", r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
