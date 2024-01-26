package main

import (
	"fmt"
	"os"

	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	log "github.com/sirupsen/logrus"
)

var AsynqClient *asynq.Client

type HadesAPIConfig struct {
	APIPort           uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	RedisConfig       utils.RedisConfig
	PrometheusAddress string `env:"PROMETHEUS_ADDRESS" envDefault:""`
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesAPIConfig
	utils.LoadConfig(&cfg)

	asynq_client_opts := asynq.RedisClientOpt{Addr: cfg.RedisConfig.Addr, Password: cfg.RedisConfig.Pwd}
	var err error
	AsynqClient = asynq.NewClient(asynq_client_opts)
	if AsynqClient == nil {
		log.WithError(err).Fatal("Failed to connect to Redis")
		return
	}

	log.Infof("Starting HadesAPI on port %d", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.ErrorLogger())
	r.Use(gin.Recovery())
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)

	h := asynqmon.New(asynqmon.Options{
		RootPath:          "/monitoring", // RootPath specifies the root for asynqmon app
		RedisConnOpt:      asynq_client_opts,
		PrometheusAddress: cfg.PrometheusAddress,
	})
	r.Any("/monitoring/*a", gin.WrapH(h))

	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
