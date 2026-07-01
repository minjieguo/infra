package cache

import (
	"context"
	"encoding/json"
	"time"
)

// SetJSON 将对象序列化为 JSON 后写入缓存。
func SetJSON[T any](ctx context.Context, key string, value T, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return Set(ctx, key, string(data), ttl)
}

// GetJSON 从缓存读取 JSON 并反序列化为指定类型。
func GetJSON[T any](ctx context.Context, key string) (T, bool, error) {
	var value T
	text, ok, err := Get(ctx, key)
	if err != nil || !ok {
		return value, ok, err
	}
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return value, false, err
	}
	return value, true, nil
}
