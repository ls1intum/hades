package main

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/Mtze/HadesCI/shared/utils"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

var AsynqClient *asynq.Client

type HadesAPIConfig struct {
	APIPort     uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	RedisConfig utils.RedisConfig
	AuthKey     string `env:"AUTH_KEY"`
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg HadesAPIConfig
	utils.LoadConfig(&cfg)

	redis_opts := asynq.RedisClientOpt{Addr: cfg.RedisConfig.Addr}
	// Check whether TLS should be enabled
	if cfg.RedisConfig.TLS_Enabled {
		redis_opts.TLSConfig = &tls.Config{}
	}
	AsynqClient = asynq.NewClient(redis_opts)
	if AsynqClient == nil {
		log.Fatal("Failed to connect to Redis")
		return
	}

	log.Infof("Starting HadesAPI on port %d", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	if cfg.AuthKey == "" {
		log.Warn("No auth key set")
	} else {
		log.Info("Auth key set")
		r.Use(gin.BasicAuth(gin.Accounts{
			"hades": cfg.AuthKey,
		}))
	}
	r.GET("/ping", ping)
	r.POST("/build", AddBuildToQueue)

	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
