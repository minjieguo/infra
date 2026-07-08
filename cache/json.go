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
func (c *Client) GetJSON(ctx context.Context, key string, value any) (bool, error) {
	text, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		return ok, err
	}
	if err := json.Unmarshal([]byte(text), value); err != nil {
		return false, err
	}
	return true, nil
}
