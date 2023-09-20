package main

import (
	"context"

	"github.com/stretchr/testify/assert"
)

func test_enqueue(t *assert.TestingT) {
	// This needs to be extended - Currently only as a reference how to use the wrapper
	q, _ := Init("builds", "amqp://guest:guest@localhost:5672/")
	defer q.Close()

	ctx := context.TODO()

	q.Enqueue(ctx, Message{Body: "Hello World", Type: 1})
}
