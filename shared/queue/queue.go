package queue

import (
	"context"
	"encoding/json"

	"github.com/Mtze/HadesCI/shared/payload"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

type Queue struct {
	channel *amqp.Channel
	queue   amqp.Queue
	conn    *amqp.Connection
}

func (q *Queue) Close() {
	log.Debugf("Closing queue %s", q.queue.Name)
	q.channel.Close()
	q.conn.Close()
}

func Init(queueName, url string) (*Queue, error) {
	var q Queue
	log.Debugf("Queue '%s' Init function called", queueName)

	var err error
	q.conn, err = amqp.Dial(url)
	if err != nil {
		log.WithError(err).Error("error connecting to RabbitMQ")
		return nil, err
	}

	q.channel, err = q.conn.Channel()
	if err != nil {
		log.WithError(err).Error("error opening RabbitMQ channel")
		return nil, err
	}

	q.queue, err = q.channel.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.WithError(err).Error("error declaring RabbitMQ queue")
		return nil, err
	}
	log.Info("Queue initialized", q)
	return &q, nil
}

func (q *Queue) Enqueue(ctx context.Context, msg payload.BuildJob) error {
	log.Debugf("Enqueue function called with ctx %+v message: %v", ctx, msg)

	body, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Error("error marshalling message")
		return err
	}

	err = q.channel.PublishWithContext(ctx,
		"",           // exchange
		q.queue.Name, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	if err != nil {
		log.WithError(err).Error("error publishing message")
		return err
	}
	return nil
}

func (q *Queue) Dequeue(callback func(<-chan amqp.Delivery)) error {
	msgs, err := q.channel.Consume(
		q.queue.Name, // queue
		"",           // consumer
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)
	if err != nil {
		log.WithError(err).Error("error consuming message")
		return err
	}

	go callback(msgs)
	return nil
}
