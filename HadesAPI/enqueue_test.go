package main

import (
	"context"

	"github.com/stretchr/testify/assert"
)

func test_enqueue(t *assert.TestingT) {
	// This needs to be extended - Currently only as a reference how to use the wrapper
	q := Queue{QueueName: "builds", URL: "amqp://guest:guest@localhost:5672/"}
	q.Init()
	defer q.Close()

	ctx := context.TODO()

	q.Enqueue(ctx, Message{Body: "Hello World", Type: 1})
}
