package utils

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type NatsConfig struct {
	URL      string `env:"NATS_URL,notEmpty" envDefault:"nats://localhost:4222"`
	Username string `env:"NATS_USERNAME"`
	Password string `env:"NATS_PASSWORD"`
	TLS      bool   `env:"NATS_TLS_ENABLED" envDefault:"false"`
}

type ExecutorConfig struct {
	// Executor is the executor to use for running the jobs (default: docker)
	// Possible values: docker, k8s
	Executor             string `env:"HADES_EXECUTOR,notEmpty" envDefault:"docker"`
	CleanupSharedVolumes bool   `env:"CLEANUP" envDefault:"false"`
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig(cfg interface{}) {
	slog.Debug("Loading config", "type", fmt.Sprintf("%T", cfg))

	if err := godotenv.Load(); err != nil {
		slog.Warn("Error loading .env file", "error", err)
	}

	if err := env.Parse(cfg); err != nil {
		slog.Error("Error parsing environment variables", "error", err)
	}

	slog.Debug("Config loaded successfully")
}

func ParseMemoryLimit(limit string) (int64, error) {
	unit := limit[len(limit)-1:]
	number := limit[:len(limit)-1]
	value, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, err
	}

	switch strings.ToUpper(unit) {
	case "g", "G":
		return value * 1024 * 1024 * 1024, nil
	case "m", "M":
		return value * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}
