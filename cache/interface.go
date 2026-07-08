package cache

import (
	"context"
	"time"
)

// Cache 缓存接口。
type Cache interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
	Close() error
}
