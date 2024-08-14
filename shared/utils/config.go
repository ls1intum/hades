package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type RedisConfig struct {
	Addr        string `env:"REDIS_ADDR,notEmpty" envDefault:"localhost:6379"`
	Pwd         string `env:"REDIS_PWD"`
	TLS_Enabled bool   `env:"REDIS_TLS_ENABLED" envDefault:"false"`
}

type ExecutorConfig struct {
	// Executor is the executor to use for running the jobs (default: docker)
	// Possible values: docker, k8s
	Executor string `env:"HADES_EXECUTOR,notEmpty" envDefault:"docker"`
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
