package cache

import (
	"context"
	"encoding/json"
	"time"
)

// SetJSON 将对象序列化为 JSON 后写入缓存。
func (c *Client) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, string(data), ttl)
}

// GetJSON 从缓存读取 JSON 并反序列化为指定类型。
// 未命中返回 ErrCacheMiss。
func (c *Client) GetJSON[T any](ctx context.Context, key string) (*T, error) {
	text, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, ErrCacheMiss
	}
	value := new(T)
	err = json.Unmarshal([]byte(text), value)
	return value, err
}
