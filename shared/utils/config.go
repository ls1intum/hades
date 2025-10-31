package utils

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

const (
	// Memory unit multipliers
	gigabyte = 1024 * 1024 * 1024
	megabyte = 1024 * 1024
)

// NatsConfig holds NATS server connection configuration.
type NatsConfig struct {
	URL      string `env:"NATS_URL,notEmpty" envDefault:"nats://localhost:4222"`
	Username string `env:"NATS_USERNAME"`
	Password string `env:"NATS_PASSWORD"`
	TLS      bool   `env:"NATS_TLS_ENABLED" envDefault:"false"`
}

// ExecutorConfig holds job executor configuration.
type ExecutorConfig struct {
	// Executor is the executor to use for running the jobs (default: docker)
	// Possible values: docker, k8s
	Executor             string `env:"HADES_EXECUTOR,notEmpty" envDefault:"docker"`
	CleanupSharedVolumes bool   `env:"CLEANUP" envDefault:"false"`
}

// LoadConfig loads configuration from environment variables and .env file.
// It will log warnings if the .env file cannot be loaded, but will not fail
// since environment variables may be provided directly.
func LoadConfig(cfg interface{}) error {
	slog.Debug("Loading config", "config", cfg)

	// Try to load .env file, but don't fail if it doesn't exist
	if err := godotenv.Load(); err != nil {
		slog.With("error", err).Warn("Error loading .env file")
	}

	// Parse environment variables into config struct
	if err := env.Parse(cfg); err != nil {
		slog.With("error", err).Error("Error parsing environment variables")
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	slog.Debug("Config loaded", "config", cfg)
	return nil
}

// ParseMemoryLimit parses a memory limit string (e.g., "1G", "512M") and returns
// the value in bytes. Supported units are G/g (gigabytes) and M/m (megabytes).
func ParseMemoryLimit(limit string) (int64, error) {
	if len(limit) < 2 {
		return 0, fmt.Errorf("invalid memory limit format: %s", limit)
	}

	unit := limit[len(limit)-1:]
	number := limit[:len(limit)-1]

	value, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value %q: %w", number, err)
	}

	if value < 0 {
		return 0, fmt.Errorf("memory limit cannot be negative: %d", value)
	}

	switch strings.ToUpper(unit) {
	case "g", "G":
		return value * gigabyte, nil
	case "m", "M":
		return value * megabyte, nil
	default:
		return 0, fmt.Errorf("unsupported memory unit %q: must be G (gigabytes) or M (megabytes)", unit)
	}
}
