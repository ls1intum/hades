package queue

import (
	"context"
	"encoding/json"

	"github.com/Mtze/HadesCI/shared/payload"
	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

const maxPriority uint8 = 5

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

	args := amqp.Table{
		"x-max-priority": maxPriority, // Set the max priority level
	}

	q.queue, err = q.channel.QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		args,      // arguments
	)
	if err != nil {
		log.WithError(err).Error("error declaring RabbitMQ queue")
		return nil, err
	}
	log.Debug("Queue initialized", q)
	return &q, nil
}

func (q *Queue) Enqueue(ctx context.Context, msg payload.QueuePayload, prio uint8) error {
	log.Debugf("Enqueue function called with ctx %+v message: %v", ctx, msg)

	if prio > maxPriority {
		log.Warnf("Priority %d is higher than max priority %d. Setting to max priority", prio, maxPriority)
		prio = maxPriority
	}

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
			ContentType: "application/json",
			Body:        []byte(body),
			Priority:    prio,
		})
	if err != nil {
		log.WithError(err).Error("error publishing message")
		return err
	}
	return nil
}

func (q *Queue) Dequeue(callback func(job payload.QueuePayload) error) error {
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

	for d := range msgs {
		var buildJob payload.QueuePayload
		err := json.Unmarshal(d.Body, &buildJob)
		if err != nil {
			log.WithError(err).Warn("error unmarshalling message %s", d.Body)
		}

		go callback(buildJob)
	}
	return nil
}
