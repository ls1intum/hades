package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type RedisConfig struct {
	Addr string `env:"REDIS_ADDR,notEmpty" envDefault:"localhost:6379"`
	Pwd  string `env:"REDIS_PWD"`
}

type K8sConfig struct {
	HadesCInamespace string `env:"HADES_CI_NAMESPACE" envDefault:"hades-ci"`
}

type ExecutorConfig struct {
	Executor string `env:"HADES_EXECUTOR,notEmpty" envDefault:"docker"`
}

func LoadConfig(cfg interface{}) {
	err := godotenv.Load()
	if err != nil {
		log.WithError(err).Warn("Error loading .env file")
	}

	err = env.Parse(cfg)
	if err != nil {
		log.WithError(err).Fatal("Error parsing environment variables")
	}

	log.Debug("Config loaded: ", cfg)
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
