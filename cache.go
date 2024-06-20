package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type Cache interface {
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (interface{}, error)
	Delete(ctx context.Context, key string) error
}

type InMemoryCache struct {
	store map[string]interface{}
}

func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{store: make(map[string]interface{})}
}

func (c *InMemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.store[key] = value
	return nil
}

func (c *InMemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	value, exists := c.store[key]
	if !exists {
		return nil, nil
	}
	return value, nil
}

func (c *InMemoryCache) Delete(ctx context.Context, key string) error {
	delete(c.store, key)
	return nil
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr, password string, db int) *RedisCache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisCache{client: rdb}
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}
