package main

import (
	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	RabbitMQUrl      string `env:"RABBITMQ_URL,notEmpty"`
	RabbitMQUser     string `env:"RABBITMQ_DEFAULT_USER,notEmpty"`
	RabbitMQPassword string `env:"RABBITMQ_DEFAULT_PASSWORD,notEmpty"`
}

var Cfg Config

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.WithError(err).Warn("Error loading .env file")
	}

	err = env.Parse(&Cfg)
	if err != nil {
		log.WithError(err).Fatal("Error parsing environment variables")
	}

	log.Debug("Config loaded: ", Cfg)
}
