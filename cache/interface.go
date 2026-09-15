package cache

import (
	"context"
	"time"
)

// Cache 缓存接口。
//
// 字符串方法与 Hash 方法并存：字符串方法以 string 直接存取，
// Hash 方法的值类型为 any，底层统一使用 JSON 编解码。
type Cache interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Close() error

	HSet(ctx context.Context, key string, field string, value any) error
	HGet(ctx context.Context, key string, field string) (any, error)
	HGetAll(ctx context.Context, key string) (map[string]any, error)
	HDel(ctx context.Context, key string, fields ...string) error
	HExists(ctx context.Context, key string, field string) (bool, error)
	HLen(ctx context.Context, key string) (int64, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	HVals(ctx context.Context, key string) ([]any, error)
	HMGet(ctx context.Context, key string, fields ...string) (map[string]any, error)
	HMSet(ctx context.Context, key string, values map[string]any) error
	HExpire(ctx context.Context, key string, ttl time.Duration, fields ...string) error
	HExpireAt(ctx context.Context, key string, at time.Time, fields ...string) error
}
