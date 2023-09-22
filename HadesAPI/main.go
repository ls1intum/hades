package main

import (
	"os"

	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var BuildQueue *queue.Queue

func main() {

	LoadConfig()

	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	// var err error
	// BuildQueue, err = queue.Init("builds", "amqp://admin:admin@localhost:5672/")
	// if err != nil {
	// 	log.Panic(err)
	// }

	log.Info("Starting HadesAPI")
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)
	log.Panic(r.Run(":8080")) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
