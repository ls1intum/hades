package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/ls1intum/hades/shared/utils"
	log "github.com/sirupsen/logrus"
)

var AsynqClient *asynq.Client

type HadesAPIConfig struct {
	APIPort           uint `env:"API_PORT,notEmpty" envDefault:"8080"`
	RedisConfig       utils.RedisConfig
	AuthKey           string `env:"AUTH_KEY"`
	PrometheusAddress string `env:"PROMETHEUS_ADDRESS" envDefault:""`
	// How long the task should be kept for monitoring
	RetentionTime uint `env:"RETENTION_IN_MIN" envDefault:"30"`
	MaxRetries    uint `env:"MAX_RETRIES" envDefault:"3"`
	Timeout       uint `env:"TIMEOUT_IN_MIN" envDefault:"0"`
}

var cfg HadesAPIConfig

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	utils.LoadConfig(&cfg)

	AsynqClient = utils.SetupQueueClient(cfg.RedisConfig.Addr, cfg.RedisConfig.Pwd, cfg.RedisConfig.TLS_Enabled)
	if AsynqClient == nil {
		return
	}

	log.Infof("Starting HadesAPI on port %d", cfg.APIPort)
	gin.SetMode(gin.ReleaseMode)

	r := setupRouter(cfg.AuthKey)

	// Start the monitoring server
	h := asynqmon.New(asynqmon.Options{
		RootPath:          "/monitoring", // RootPath specifies the root for asynqmon app
		RedisConnOpt:      asynq.RedisClientOpt{Addr: cfg.RedisConfig.Addr, Password: cfg.RedisConfig.Pwd},
		PrometheusAddress: cfg.PrometheusAddress,
		PayloadFormatter:  MetadataObfuscator,
	})
	r.Any("/monitoring/*a", gin.WrapH(h))

	log.Panic(r.Run(fmt.Sprintf(":%d", cfg.APIPort)))
}
