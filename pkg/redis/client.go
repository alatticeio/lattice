package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	_ "github.com/redis/go-redis/v9"
	"time"
)

type Client struct {
	rdb *redis.Client
}

type ClientConfig struct {
	Addr     string
	Password string
	DB       int
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password, // no password set
		DB:       0,
	})

	ctx := context.Background()

	return &Client{rdb: rdb}, rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// Get returns the value of the key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

func (c *Client) Set(ctx context.Context, key string, value interface{}) error {
	return c.rdb.Set(ctx, key, value, 0).Err()
}

func (c *Client) SetWithExpire(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

func (c *Client) Del(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}
