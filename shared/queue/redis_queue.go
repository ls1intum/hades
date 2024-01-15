package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

type RedisQueue struct {
	client    *redis.Client
	queueName string
}

func (q *RedisQueue) Close() {
	q.client.Close()
}

func InitRedis(queueName, url string) (*RedisQueue, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     url,
		Password: "",
		DB:       0,
	})
	return &RedisQueue{redisClient, queueName}, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, msg payload.QueuePayload, prio uint8) error {
	timestamp := float64(time.Now().UnixNano()) / 1e9
	score := float64(prio) + timestamp/1e10

	body, err := json.Marshal(msg)
	if err != nil {
		log.WithError(err).Error("error marshalling message")
		return err
	}

	tmp := q.client.ZAdd(ctx, q.queueName, redis.Z{
		Score:  score,
		Member: body,
	})
	return tmp.Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context, callback func(job payload.QueuePayload) error) error {
	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping job processing")
			return nil
		default:
			log.Info("Waiting for job...")
			ctx := context.Background()
			jobJSON, err := q.client.BZPopMin(ctx, 0, q.queueName).Result()
			if err != nil {
				log.Warn("Error waiting for job:", err)
				continue
			}

			jobString, ok := jobJSON.Member.(string) // Assert to string
			if !ok {
				log.Warn("Job is not a string")
				continue
			}

			var payload payload.QueuePayload
			err = json.Unmarshal([]byte(jobString), &payload)
			if err != nil {
				log.WithError(err).Warn("error unmarshalling message")
			}

			log.Debug("Received job ", payload)
			err = callback(payload)
			log.WithError(err).Warnf("Error executing job %s", payload.Name)
		}
	}
}
