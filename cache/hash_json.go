package cache

import (
	"context"
	"encoding/json"
)

// HSetJSON 将对象序列化为 JSON 后写入 Hash 的指定字段。
//
// 注意与 HGet 的区别：本方法避免了 HGet 的双层 JSON 解码，
// 写入的文本按原样读取，可无损还原为原始类型。
func HSetJSON[T any](c *Client, ctx context.Context, key string, field string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.HSet(ctx, key, field, string(data))
}

// HGetJSON 从 Hash 的指定字段读取 JSON 并反序列化为指定类型。
// 字段不存在返回 ErrCacheMiss。
//
// 这里通过 HGetAll 取值而非 HGet：HGet 会把字段文本再解码一次，
// 导致 "alice" 这类 JSON 字符串被还原成去掉引号的 alice，
// 再反序列化到 string 字段时丢失原始内容。使用 HGetAll 可拿到原始文本。
func HGetJSON[T any](c *Client, ctx context.Context, key string, field string) (*T, error) {
	items, err := c.HGetAll(ctx, key)
	if err != nil {
		return nil, err
	}
	text, ok := rawHashText(items[field])
	if !ok {
		return nil, ErrCacheMiss
	}
	value := new(T)
	if err := json.Unmarshal([]byte(text), value); err != nil {
		return nil, err
	}
	return value, nil
}

// rawHashText 从 HGetAll 的结果中还原字段的原始文本。
//
// HGetAll 内部已做一次 JSON 解码，
// 因此字符串字段会表现为 Go 的 string（引号已被剥离），
// 这里需要把可能被剥离的引号补回，才能作为合法 JSON 再次解码。
func rawHashText(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		// 非字符串类型（数字、布尔、对象等）重新编码即可
		data, err := json.Marshal(value)
		if err != nil {
			return "", false
		}
		return string(data), true
	}
	// 字符串需要重新加上引号，否则 json.Unmarshal 到 string 字段会失败
	quoted, err := json.Marshal(text)
	if err != nil {
		return "", false
	}
	return string(quoted), true
}
