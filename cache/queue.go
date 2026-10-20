package cache

import (
	"context"
	"errors"
	"fmt"
)

// ErrQueueClosed 表示队列所在的缓存存储已关闭，阻塞中的 Pop 会被唤醒并返回该错误。
var ErrQueueClosed = errors.New("cache queue closed")

// Push 从队列左侧写入一个元素，对应 Redis 的 LPUSH。
func (c *Client) Push(ctx context.Context, key string, value string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.LPush(ctx, key, value)
}

// Pop 从队列右侧阻塞取出一个元素，对应 Redis 的 BRPOP。
//
// 队列为空时一直阻塞，直到有元素入队、ctx 被取消或客户端关闭；
// ctx 结束时返回 ctx.Err()，客户端关闭时返回 ErrQueueClosed。
func (c *Client) Pop(ctx context.Context, key string) (string, error) {
	if c == nil || c.store == nil {
		return "", fmt.Errorf("cache is not initialized")
	}
	return c.store.BRPop(ctx, key)
}
