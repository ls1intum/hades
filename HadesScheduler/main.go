package main

import (
	"os"

	"github.com/Mtze/HadesCI/hadesScheduler/docker"
	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/Mtze/HadesCI/shared/utils"

	log "github.com/sirupsen/logrus"
)

var JobQueue queue.JobQueue

type JobScheduler interface {
	ScheduleJob(job payload.QueuePayload) error
}

func main() {
	if is_debug := os.Getenv("DEBUG"); is_debug == "true" {
		log.SetLevel(log.DebugLevel)
		log.Warn("DEBUG MODE ENABLED")
	}

	var cfg utils.RedisConfig
	utils.LoadConfig(&cfg)

	var executorCfg utils.ExecutorConfig
	utils.LoadConfig(&executorCfg)
	log.Debug("Executor config: ", executorCfg)

	var err error
	JobQueue, err = queue.InitRedis("builds", cfg.Addr)

	if err != nil {
		log.Panic(err)
	}

	var forever chan struct{}

	var scheduler JobScheduler

	switch executorCfg.Executor {
	// case "k8s":
	// 	log.Info("Started HadesScheduler in Kubernetes mode")
	// 	kube.Init()
	// 	scheduler = kube.Scheduler{}
	case "docker":
		log.Info("Started HadesScheduler in Docker mode")
		scheduler = docker.Scheduler{}
	default:
		log.Fatalf("Invalid executor specified: %s", executorCfg.Executor)
	}

	JobQueue.Dequeue(scheduler.ScheduleJob)

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

}
