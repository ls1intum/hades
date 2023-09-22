package main

import (
	"fmt"

	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var BuildQueue *queue.Queue

type HadesAPIConfig struct {
	APIPort        uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	RabbitMQConfig utils.RabbitMQConfig
}

func main() {

	var cfg HadesAPIConfig
	utils.LoadConfig(&cfg)

	var err error
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s/", cfg.RabbitMQConfig.User, cfg.RabbitMQConfig.Password, cfg.RabbitMQConfig.Url)
	log.Debug("Connecting to RabbitMQ: ", rabbitmqURL)
	BuildQueue, err = queue.Init("builds", "amqp://admin:admin@localhost:5672/")
	if err != nil {
		log.Panic(err)
	}

	log.Info("Starting HadesAPI")
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)
	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort))) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
