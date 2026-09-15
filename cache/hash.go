package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// marshalHashValue 将 Hash 字段的任意值编码为字符串。
//
// 字符串原样落库，便于与 Redis 命令行核对数据；
// 其他类型统一走 JSON 编码。
func marshalHashValue(value any) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// unmarshalHashValue 将 Hash 字段的字符串还原为任意值。
//
// 优先尝试 JSON 解码；非 JSON 文本（如手工写入的纯字符串）按字符串返回。
func unmarshalHashValue(text string) (any, error) {
	var value any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return text, nil
	}
	return value, nil
}

// HSet 写入 Hash 的单个字段。
//
// 不设置键的过期时间，需要过期时显式调用 HExpire 或 HExpireAt。
func (c *Client) HSet(ctx context.Context, key string, field string, value any) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.HSet(ctx, key, field, value)
}

// HGet 读取 Hash 的单个字段。字段不存在返回 ErrCacheMiss。
func (c *Client) HGet(ctx context.Context, key string, field string) (any, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.HGet(ctx, key, field)
}

// HGetAll 读取整个 Hash。键不存在返回空 map。
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]any, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.HGetAll(ctx, key)
}

// HDel 删除 Hash 的一个或多个字段。
func (c *Client) HDel(ctx context.Context, key string, fields ...string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.HDel(ctx, key, fields...)
}

// HExists 判断 Hash 字段是否存在。
func (c *Client) HExists(ctx context.Context, key string, field string) (bool, error) {
	if c == nil || c.store == nil {
		return false, fmt.Errorf("cache is not initialized")
	}
	return c.store.HExists(ctx, key, field)
}

// HLen 返回 Hash 的字段数量。
func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.HLen(ctx, key)
}

// HKeys 返回 Hash 的全部字段名。键不存在返回空切片。
func (c *Client) HKeys(ctx context.Context, key string) ([]string, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.HKeys(ctx, key)
}

// HVals 返回 Hash 的全部字段值。键不存在返回空切片。
func (c *Client) HVals(ctx context.Context, key string) ([]any, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.HVals(ctx, key)
}

// HMGet 批量读取 Hash 字段。不存在的字段不会出现在结果中。
func (c *Client) HMGet(ctx context.Context, key string, fields ...string) (map[string]any, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.HMGet(ctx, key, fields...)
}

// HMSet 批量写入 Hash 字段。
//
// 不设置键的过期时间，需要过期时显式调用 HExpire 或 HExpireAt。
func (c *Client) HMSet(ctx context.Context, key string, values map[string]any) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.HMSet(ctx, key, values)
}

// HExpire 为 Hash 的指定字段设置相对过期时间。
//
// 依赖 Redis 7.4+ 的 HEXPIRE 命令，字段级 TTL；
// memory 实现不支持字段级过期，直接返回成功。
func (c *Client) HExpire(ctx context.Context, key string, ttl time.Duration, fields ...string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.HExpire(ctx, key, ttl, fields...)
}

// HExpireAt 为 Hash 的指定字段设置绝对过期时间。
//
// 依赖 Redis 7.4+ 的 HEXPIREAT 命令，语义同 HExpire。
func (c *Client) HExpireAt(ctx context.Context, key string, at time.Time, fields ...string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.HExpireAt(ctx, key, at, fields...)
}
