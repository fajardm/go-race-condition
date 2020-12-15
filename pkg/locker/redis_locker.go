package locker

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

type redisLocker struct {
	client *redis.Client
}

func (l *redisLocker) Lock(ctx context.Context, key string, expiration time.Duration) (err error) {
	res, err := l.client.GetSet(ctx, key, 1).Result()
	if err != nil && err != redis.Nil {
		return
	}

	if expiration != 0 {
		if _, err = l.client.Expire(ctx, key, expiration).Result(); err != nil {
			return
		}
	}

	if res != "" {
		err = ErrLocked
	}
	return
}

func (l *redisLocker) Unlock(ctx context.Context, key string) (err error) {
	res, err := l.client.Del(ctx, key).Result()
	if res == 0 {
		err = ErrLocked
	}
	return
}

func (l *redisLocker) Locked(ctx context.Context, key string) (res bool) {
	n, err := l.client.Exists(ctx, key).Result()
	res = n > 0 && err == nil
	return
}

func NewRedisLocker(client *redis.Client) Locker {
	return &redisLocker{client: client}
}
