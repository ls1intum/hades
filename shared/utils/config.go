package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
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

type JetStreamJobsConfig struct {
	StreamName string                    `env:"JETSTREAM_JOBS_STREAM_NAME"`
	Subjects   []string                  `env:"JETSTREAM_JOBS_SUBJECTS"`
	Storage    jetstream.StorageType     `env:"JETSTREAM_JOBS_STORAGE"`
	Retention  jetstream.RetentionPolicy `env:"JETSTREAM_JOBS_RETENTION"`
	Duplicates time.Duration             `env:"JETSTREAM_JOBS_DUPLICATES"`
	MaxMsgs    int64                     `env:"JETSTREAM_JOBS_MAX_MSGS"`
	MaxAge     time.Duration             `env:"JETSTREAM_JOBS_MAX_AGE"`
}

type JetStreamJobLogsConfig struct {
	StreamName string                    `env:"JETSTREAM_LOGS_STREAM_NAME"`
	Subjects   []string                  `env:"JETSTREAM_LOGS_SUBJECTS"`
	Storage    jetstream.StorageType     `env:"JETSTREAM_LOGS_STORAGE"`
	Retention  jetstream.RetentionPolicy `env:"JETSTREAM_LOGS_RETENTION"`
	Duplicates time.Duration             `env:"JETSTREAM_LOGS_DUPLICATES"`
	MaxMsgs    int64                     `env:"JETSTREAM_LOGS_MAX_MSGS"`
	MaxAge     time.Duration             `env:"JETSTREAM_LOGS_MAX_AGE"`
}

func LoadConfig(cfg interface{}) {
	log.Debug("Loading config for: ", "config", cfg)

	err := godotenv.Load()
	if err != nil {
		log.With("error", err).Warn("Error loading .env file")
	}

	err = env.Parse(cfg)
	if err != nil {
		log.With("error", err).Error("Error parsing environment variables")
	}

	log.Debug("Config loaded", "config", cfg)
}

func (c JetStreamJobLogsConfig) ToStreamConfig() jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:       c.StreamName,
		Subjects:   c.Subjects,
		Storage:    c.Storage,
		Retention:  c.Retention,
		Duplicates: c.Duplicates,
		MaxMsgs:    c.MaxMsgs,
		MaxAge:     c.MaxAge,
	}
}

func (c JetStreamJobsConfig) ToStreamConfig() jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:       c.StreamName,
		Subjects:   c.Subjects,
		Storage:    c.Storage,
		Retention:  c.Retention,
		Duplicates: c.Duplicates,
		MaxMsgs:    c.MaxMsgs,
		MaxAge:     c.MaxAge,
	}
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
