package cache

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	TypeMemory = "memory"
	TypeRedis  = "redis"
)

// Config 缓存配置。
type Config struct {
	Type     string
	Addr     string
	Password string
	DB       int
	Prefix   string
}

// Client 缓存客户端。
type Client struct {
	store Cache
}

// New 初始化缓存客户端。
func New(cfg Config) (*Client, error) {
	cacheType := strings.TrimSpace(strings.ToLower(cfg.Type))
	if cacheType == "" {
		cacheType = TypeMemory
	}

	var store Cache
	switch cacheType {
	case TypeMemory:
		store = newMemoryStore()
	case TypeRedis:
		redisStore, err := newRedisStore(cfg)
		if err != nil {
			return nil, err
		}
		store = redisStore
	default:
		return nil, fmt.Errorf("undefined cache type:%s", cfg.Type)
	}
	return &Client{store: store}, nil
}

// Set 写入缓存。
func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.Set(ctx, key, value, ttl)
}

// Get 读取缓存。
func (c *Client) Get(ctx context.Context, key string) (string, bool, error) {
	if c == nil || c.store == nil {
		return "", false, fmt.Errorf("cache is not initialized")
	}
	return c.store.Get(ctx, key)
}

// Delete 删除缓存。
func (c *Client) Delete(ctx context.Context, key string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.Delete(ctx, key)
}

// Close 关闭缓存客户端。
func (c *Client) Close() error {
	if c == nil || c.store == nil {
		return nil
	}
	return c.store.Close()
}
