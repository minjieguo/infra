package cache

import (
	"context"
	"time"
)

// Store 缓存接口。
type Store interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, bool, error)
	Delete(ctx context.Context, key string) error
	Close() error
}
