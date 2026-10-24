package cache

import (
	"context"
	"fmt"
	"sort"
)

// ZSetMember 有序集合成员及其分值。
type ZSetMember struct {
	Member string  // Member 成员名
	Score  float64 // Score 分值
}

// ZAdd 批量写入有序集合成员及其分值，已存在的成员会被更新分值。
//
// 对应 Redis 的 ZADD 命令；分值为 float64，不做取整。
func (c *Client) ZAdd(ctx context.Context, key string, members map[string]float64) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.ZAdd(ctx, key, members)
}

// ZRange 按分值升序返回 [start, stop] 区间的成员。
//
// 对应 Redis 的 ZRANGE 命令；下标从 0 开始，支持负数（-1 表示最后一个），
// 越界区间会被自动裁剪，集合不存在时返回空切片。
func (c *Client) ZRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZRange(ctx, key, start, stop)
}

// ZRevRange 按分值降序返回 [start, stop] 区间的成员。
//
// 对应 Redis 的 ZREVRANGE 命令，其余语义与 ZRange 一致。
func (c *Client) ZRevRange(ctx context.Context, key string, start int64, stop int64) ([]ZSetMember, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZRevRange(ctx, key, start, stop)
}

// ZCard 返回有序集合的成员数量，集合不存在时返回 0。
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZCard(ctx, key)
}

// ZScore 返回指定成员的分值，成员不存在时 exists 为 false。
func (c *Client) ZScore(ctx context.Context, key string, member string) (float64, bool, error) {
	if c == nil || c.store == nil {
		return 0, false, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZScore(ctx, key, member)
}

// ZIncrBy 将指定成员的分值累加 delta 并返回累加后的分值。
//
// 成员不存在时按 delta 创建，等价于 Redis 的 ZINCRBY。
func (c *Client) ZIncrBy(ctx context.Context, key string, member string, delta float64) (float64, error) {
	if c == nil || c.store == nil {
		return 0, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZIncrBy(ctx, key, member, delta)
}

// ZRem 批量删除有序集合成员，成员不存在时忽略。
func (c *Client) ZRem(ctx context.Context, key string, members ...string) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("cache is not initialized")
	}
	return c.store.ZRem(ctx, key, members...)
}

// ZRank 返回成员按分值升序的名次（从 0 开始），成员不存在时 exists 为 false。
func (c *Client) ZRank(ctx context.Context, key string, member string) (int64, bool, error) {
	if c == nil || c.store == nil {
		return 0, false, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZRank(ctx, key, member)
}

// ZRevRank 返回成员按分值降序的名次（从 0 开始），成员不存在时 exists 为 false。
func (c *Client) ZRevRank(ctx context.Context, key string, member string) (int64, bool, error) {
	if c == nil || c.store == nil {
		return 0, false, fmt.Errorf("cache is not initialized")
	}
	return c.store.ZRevRank(ctx, key, member)
}

// sortZSetMembers 将成员集合按分值升序排序，同分按成员名字典序升序。
//
// 与 Redis 的排序规则保持一致，保证 memory 与 redis 后端结果相同。
func sortZSetMembers(scores map[string]float64) []ZSetMember {
	members := make([]ZSetMember, 0, len(scores))
	for member, score := range scores {
		members = append(members, ZSetMember{Member: member, Score: score})
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].Score != members[j].Score {
			return members[i].Score < members[j].Score
		}
		return members[i].Member < members[j].Member
	})
	return members
}

// sliceZSetMembers 按 Redis 的下标语义裁剪有序成员切片。
//
// start/stop 支持负数下标，越界部分被裁剪；区间非法时返回空切片。
func sliceZSetMembers(members []ZSetMember, start int64, stop int64) []ZSetMember {
	size := int64(len(members))
	if size == 0 {
		return []ZSetMember{}
	}
	if start < 0 {
		start += size
	}
	if stop < 0 {
		stop += size
	}
	if start < 0 {
		start = 0
	}
	if stop >= size {
		stop = size - 1
	}
	if start > stop {
		return []ZSetMember{}
	}
	result := make([]ZSetMember, stop-start+1)
	copy(result, members[start:stop+1])
	return result
}

// reverseZSetMembers 返回成员切片的降序副本。
func reverseZSetMembers(members []ZSetMember) []ZSetMember {
	size := len(members)
	result := make([]ZSetMember, size)
	for i, member := range members {
		result[size-1-i] = member
	}
	return result
}

// zSetRank 返回成员在升序切片中的下标，未找到时 exists 为 false。
func zSetRank(members []ZSetMember, member string) (int64, bool) {
	for i, item := range members {
		if item.Member == member {
			return int64(i), true
		}
	}
	return 0, false
}
