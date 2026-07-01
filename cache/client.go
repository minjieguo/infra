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

var client Store

// New 初始化缓存客户端。
func New(cfg Config) error {
	cacheType := strings.TrimSpace(strings.ToLower(cfg.Type))
	if cacheType == "" {
		cacheType = TypeMemory
	}

	switch cacheType {
	case TypeMemory:
		client = newMemoryStore()
	case TypeRedis:
		store, err := newRedisStore(cfg)
		if err != nil {
			return err
		}
		client = store
	default:
		return fmt.Errorf("undefined cache type:%s", cfg.Type)
	}
	return nil
}

// Set 写入缓存。
func Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if client == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return client.Set(ctx, key, value, ttl)
}

// Get 读取缓存。
func Get(ctx context.Context, key string) (string, bool, error) {
	if client == nil {
		return "", false, fmt.Errorf("cache is not initialized")
	}
	return client.Get(ctx, key)
}

// Delete 删除缓存。
func Delete(ctx context.Context, key string) error {
	if client == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return client.Delete(ctx, key)
}

// Close 关闭缓存客户端。
func Close() error {
	if client == nil {
		return nil
	}
	return client.Close()
}
