package utils

import (
	"crypto/tls"

	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func SetupQueueClient(redis_addr string, redis_pwd string, tls_enabled bool) *asynq.Client {
	redis_opts := asynq.RedisClientOpt{Addr: redis_addr, Password: redis_pwd}
	// Check whether TLS should be enabled
	if tls_enabled {
		redis_opts.TLSConfig = &tls.Config{}
	}
	asynqClient := asynq.NewClient(redis_opts)
	if asynqClient == nil {
		log.Fatal("Failed to connect to Redis")
		return nil
	}
	return asynqClient
}

func SetupQueueServer(redis_addr string, redis_pwd string, tls_enabled bool, concurrency int) *asynq.Server {
	redis_opts := asynq.RedisClientOpt{Addr: redis_addr, Password: redis_pwd}
	// Check whether TLS should be enabled
	if tls_enabled {
		redis_opts.TLSConfig = &tls.Config{}
	}
	asynqServer := asynq.NewServer(redis_opts, asynq.Config{
		Concurrency: concurrency,
		Queues: map[string]int{
			"critical": 5,
			"high":     4,
			"normal":   3,
			"low":      2,
			"minimal":  1,
		},
		StrictPriority: true,
		Logger:         log.StandardLogger(),
	})
	if asynqServer == nil {
		log.Fatal("Failed to create Asynq server")
		return nil
	}
	return asynqServer
}
