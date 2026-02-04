package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	hades "github.com/ls1intum/hades/shared"
	hadesnats "github.com/ls1intum/hades/shared/nats"
	"github.com/ls1intum/hades/shared/utils"
	log "github.com/sirupsen/logrus"
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
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	utils.LoadConfig(&cfg)

	var err error
	NatsConnection, err := hadesnats.SetupNatsConnection(cfg.NatsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
		return
	}
	defer NatsConnection.Close()

	HadesProducer, err = hadesnats.NewHadesPublisher(NatsConnection)
	if err != nil {
		log.Fatalf("Failed to create HadesProducer: %v", err)
		return
	}

	log.Infof("Starting HadesAPI on port %d", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := setupRouter(cfg.AuthKey)

	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
