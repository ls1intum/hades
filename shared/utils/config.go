package utils

import (
	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type RedisConfig struct {
	Addr        string `env:"REDIS_ADDR,notEmpty" envDefault:"localhost:6379"`
	Pwd         string `env:"REDIS_PWD"`
	TLS_Enabled bool   `env:"REDIS_TLS_ENABLED" envDefault:"false"`
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
