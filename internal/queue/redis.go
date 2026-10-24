package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/miebyte/emailsender/internal/email"
	"github.com/miebyte/goutils/redisutils"
	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client  *redisutils.RedisClient
	name    string
	timeout time.Duration
}

var _ email.TaskQueue = (*RedisQueue)(nil)

func NewRedisQueue(client *redisutils.RedisClient, name string, timeout time.Duration) (*RedisQueue, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("email queue name is required")
	}

	if timeout <= 0 {
		return nil, errors.New("redis timeout must be positive")
	}
	return &RedisQueue{client: client, name: name, timeout: timeout}, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, task email.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	operationCtx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()

	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("encode email task: %w", err)
	}

	if err := q.client.RPush(operationCtx, q.name, data).Err(); err != nil {
		return fmt.Errorf("enqueue email task: %w", queueError(ctx, operationCtx, err))
	}
	return nil
}

func (q *RedisQueue) Dequeue(ctx context.Context) (email.Task, error) {
	if err := ctx.Err(); err != nil {
		return email.Task{}, err
	}
	operationCtx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	result, err := q.client.BLPop(operationCtx, time.Second, q.name).Result()
	if err != nil {
		err = queueError(ctx, operationCtx, err)
		if errors.Is(err, redis.Nil) {
			return email.Task{}, email.ErrQueueEmpty
		}
		return email.Task{}, fmt.Errorf("dequeue email task: %w", err)
	}
	if len(result) != 2 {
		return email.Task{}, errors.New("invalid email queue response")
	}
	var task email.Task
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		return email.Task{}, fmt.Errorf("decode email task: %w", err)
	}
	if task.ID < 1 {
		return email.Task{}, errors.New("invalid email task ID")
	}
	return task, nil
}

func queueError(ctx, operationCtx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if deadline, ok := operationCtx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return err
}
