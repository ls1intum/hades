package main

import (
	"fmt"
	"os"

	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var BuildQueue *queue.Queue
var MonitorClient *MonitoringClient

type HadesAPIConfig struct {
	APIPort        uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	RabbitMQConfig utils.RabbitMQConfig
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesAPIConfig
	utils.LoadConfig(&cfg)

	var err error
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s/", cfg.RabbitMQConfig.User, cfg.RabbitMQConfig.Password, cfg.RabbitMQConfig.Url)
	log.Debug("Connecting to RabbitMQ: ", rabbitmqURL)
	BuildQueue, err = queue.Init("builds", rabbitmqURL)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to RabbitMQ")
		return
	}

	MonitorClient, err = NewMonitoringClient(cfg.RabbitMQConfig.Url, cfg.RabbitMQConfig.User, cfg.RabbitMQConfig.Password)
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to RabbitMQ Management API")
		return
	}

	log.Infof("Starting HadesAPI on port %d", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)
	r.GET("/monitoring", MonitoringQueue)

	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
