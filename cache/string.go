package cache

import (
	"context"
	"fmt"
	"time"
)

// TTLNoExpire 表示键存在但没有设置过期时间。
//
// 对应 Redis 的 TTL 返回 -1；这里用 time.Duration 表达，
// 取 -1 毫秒以便与「真实剩余时长」区分。
const TTLNoExpire = -1 * time.Millisecond

// DeleteMany 批量删除缓存键，键不存在时忽略。
func (c *Client) DeleteMany(ctx context.Context, keys ...string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.DeleteMany(ctx, keys...)
}

// Exists 判断缓存键是否存在。
//
// 对字符串、Hash、有序集合、队列四类数据结构均生效。
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.store == nil {
		return false, fmt.Errorf("cache is not initialized")
	}
	return c.store.Exists(ctx, key)
}

// Expire 为已存在的键设置相对过期时间，键不存在时不做任何处理。
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.Expire(ctx, key, ttl)
}

// TTL 返回键的剩余过期时间。
//
// 键不存在时返回 ErrCacheMiss；键存在但未设置过期时间时返回 TTLNoExpire。
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.TTL(ctx, key)
}

// IncrBy 将键的整数值累加 delta 并返回累加后的值，键不存在时按 0 起算。
//
// 注意与 Set 的区别：memory 后端的计数值独立存放，
// 不会被 Set 写入的字符串影响；redis 后端则与 Redis 一致，
// 键值不是整数时返回底层错误。
func (c *Client) IncrBy(ctx context.Context, key string, delta int64) (int64, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.IncrBy(ctx, key, delta)
}

// DecrBy 将键的整数值减去 delta 并返回计算后的值，键不存在时按 0 起算。
func (c *Client) DecrBy(ctx context.Context, key string, delta int64) (int64, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.DecrBy(ctx, key, delta)
}
