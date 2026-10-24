package cache

import (
	"context"
	"time"
)

// Cache 缓存接口。
//
// 字符串方法与 Hash 方法并存：字符串方法以 string 直接存取，
// Hash 方法的值类型为 any，底层统一使用 JSON 编解码。
// 有序集合方法以 ZSetMember 承载 member 与 score。
type Cache interface {
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	DeleteMany(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	IncrBy(ctx context.Context, key string, delta int64) (int64, error)
	DecrBy(ctx context.Context, key string, delta int64) (int64, error)
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

	LPush(ctx context.Context, key string, value string) error
	BRPop(ctx context.Context, key string) (string, error)

	ZAdd(ctx context.Context, key string, members map[string]float64) error
	ZRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error)
	ZRevRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error)
	ZRangeByScore(ctx context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error)
	ZRevRangeByScore(ctx context.Context, key string, min float64, max float64, offset int64, count int64) ([]ZSetMember, error)
	ZCard(ctx context.Context, key string) (int64, error)
	ZCount(ctx context.Context, key string, min float64, max float64) (int64, error)
	ZScore(ctx context.Context, key string, member string) (float64, bool, error)
	ZIncrBy(ctx context.Context, key string, member string, delta float64) (float64, error)
	ZRem(ctx context.Context, key string, members ...string) error
	ZRemRangeByScore(ctx context.Context, key string, min float64, max float64) (int64, error)
	ZRemRangeByRank(ctx context.Context, key string, start int64, stop int64) (int64, error)
	ZRank(ctx context.Context, key string, member string) (int64, bool, error)
	ZRevRank(ctx context.Context, key string, member string) (int64, bool, error)
}
