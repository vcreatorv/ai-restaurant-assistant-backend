package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Redis обёртка над go-redis для key-value операций
type Redis interface {
	// Set записывает значение с TTL
	Set(ctx context.Context, key, value string, expiration time.Duration) error
	// Get возвращает значение по ключу
	Get(ctx context.Context, key string) (string, error)
	// Del удаляет ключ
	Del(ctx context.Context, key string) error
	// Expire обновляет TTL ключа
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// Close закрывает соединение
	Close() error
}

type redisManager struct {
	client *goredis.Client
}

// NewRedis создаёт Redis-обёртку над go-redis Client
func NewRedis(client *goredis.Client) Redis {
	return &redisManager{client: client}
}

// Set записывает значение с TTL
func (r *redisManager) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get возвращает значение по ключу
func (r *redisManager) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Del удаляет ключ
func (r *redisManager) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Expire обновляет TTL ключа
func (r *redisManager) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// Close закрывает соединение
func (r *redisManager) Close() error {
	return r.client.Close()
}
