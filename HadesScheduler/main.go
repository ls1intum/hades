package main

import (
	"fmt"

	"github.com/Mtze/HadesCI/hadesScheduler/kube"
	"github.com/Mtze/HadesCI/shared/queue"
	"github.com/Mtze/HadesCI/shared/utils"

	log "github.com/sirupsen/logrus"
)

var BuildQueue *queue.Queue

func main() {
	var cfg utils.RabbitMQConfig
	utils.LoadConfig(&cfg)

	var err error
	rabbitmqURL := fmt.Sprintf("amqp://%s:%s@%s/", cfg.User, cfg.Password, cfg.Url)
	log.Debug("Connecting to RabbitMQ: ", rabbitmqURL)
	BuildQueue, err = queue.Init("builds", rabbitmqURL)

	if err != nil {
		log.Panic(err)
	}

	var forever chan struct{}

	scheduler := kube.Scheduler{}
	BuildQueue.Dequeue(scheduler.ScheduleJob)

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever

}
