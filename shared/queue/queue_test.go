package queue

import (
	"context"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func test_enqueue(t *assert.TestingT) {
	// This needs to be extended - Currently only as a reference how to use the wrapper
	q, err := Init[TypedMessage]("builds", "amqp://admin:admin@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	defer q.Close()

	ctx := context.TODO()

	q.Enqueue(ctx, TypedMessage{Body: "Hello World", Type: 1})

	f := func(ch <-chan amqp.Delivery) {
		for d := range ch {
			log.Printf("Received a message: %s", d.Body)
		}
	}
	q.Dequeue(f)
}
