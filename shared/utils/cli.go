package utils

import (
	"os"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	RabbitMQUrl      string `env:"RABBITMQ_URL,notEmpty"`
	RabbitMQUser     string `env:"RABBITMQ_DEFAULT_USER,notEmpty"`
	RabbitMQPassword string `env:"RABBITMQ_DEFAULT_PASS,notEmpty"`
}

var Cfg Config

func LoadConfig(cfg interface{}) {

	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

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
