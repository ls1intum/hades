package main

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

type Message struct {
	Body string `json:"body"`
	Type int    `json:"type"`
}

type Queue struct {
	QueueName string
	URL       string // amqp://guest:guest@localhost:5672/
	channel   *amqp.Channel
	queue     amqp.Queue
	conn      *amqp.Connection
}

func (q *Queue) Close() {
	log.Debugf("Closing queue %s", q.QueueName)
	q.channel.Close()
	q.conn.Close()
}

func (q *Queue) Init() {
	log.Debugf("Queue '%s' Init function called", q.QueueName)

	var err error

	q.conn, err = amqp.Dial(q.URL)
	if err != nil {
		log.WithError(err).Error("error connecting to RabbitMQ")
	}

	q.channel, err = q.conn.Channel()
	if err != nil {
		log.WithError(err).Error("error opening RabbitMQ channel")
	}

	q.queue, err = q.channel.QueueDeclare(
		q.QueueName, // name
		false,       // durable
		false,       // delete when unused
		false,       // exclusive
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		log.WithError(err).Error("error declaring RabbitMQ queue")
	}
	log.Info("Queue initialized", q)
}

func (q *Queue) Enqueue(ctx context.Context, msg Message) {
	log.Debugf("Enqueue function called with ctx %+v message: %v", ctx, msg)

	body, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Error("error marshalling message")
	}

	err = q.channel.PublishWithContext(ctx,
		"",          // exchange
		q.QueueName, // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	if err != nil {
		log.WithError(err).Error("error publishing message")
	}

}
