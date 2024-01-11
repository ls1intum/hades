package queue

import (
	"context"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client *redis.Client
}

func (q *RedisQueue) Close() {

}

func InitRedis(queueName, url string) (*RedisQueue, error) {
	return nil, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, msg payload.QueuePayload, prio uint8) error {

	return nil
}

func (q *RedisQueue) Dequeue(callback func(job payload.QueuePayload) error) error {

	return nil
}
