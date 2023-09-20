package main

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Info("Starting HadesAPI")
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)
	log.Panic(r.Run(":8080")) // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
