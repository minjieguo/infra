package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// testZSetKey 生成本次用例唯一的 zset key，避免用例之间互相污染。
func testZSetKey() string {
	return fmt.Sprintf("zset-%d", time.Now().UnixNano())
}

// newTestMemoryClient 创建内存后端客户端，用于与 Redis 行为做对照。
func newTestMemoryClient(t *testing.T) *Client {
	t.Helper()

	client, err := New(Config{Type: TypeMemory})
	if err != nil {
		t.Fatalf("创建内存缓存客户端失败: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

// zsetMembersToStrings 将有序集合结果转换为 "member:score" 便于断言。
func zsetMembersToStrings(members []ZSetMember) []string {
	result := make([]string, 0, len(members))
	for _, item := range members {
		result = append(result, fmt.Sprintf("%s:%g", item.Member, item.Score))
	}
	return result
}

// assertMembers 比较有序集合结果与期望的 "member:score" 序列。
func assertMembers(t *testing.T, label string, got []ZSetMember, want []string) {
	t.Helper()

	gotText := zsetMembersToStrings(got)
	if len(gotText) != len(want) {
		t.Fatalf("%s 长度不匹配: want=%v got=%v", label, want, gotText)
	}
	for i := range want {
		if gotText[i] != want[i] {
			t.Fatalf("%s 顺序不匹配: want=%v got=%v", label, want, gotText)
		}
	}
}

// runZSetCases 对给定后端执行同一套有序集合断言，保证 memory 与 redis 行为一致。
func runZSetCases(t *testing.T, client *Client) {
	t.Helper()

	ctx := context.Background()
	key := testZSetKey()

	// 同分成员按名字典序升序
	if err := client.ZAdd(ctx, key, map[string]float64{
		"b": 10,
		"a": 10,
		"c": 5,
		"d": 20,
	}); err != nil {
		t.Fatalf("ZAdd 失败: %v", err)
	}

	all, err := client.ZRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRange 失败: %v", err)
	}
	assertMembers(t, "ZRange 全量", all, []string{"c:5", "a:10", "b:10", "d:20"})

	rev, err := client.ZRevRange(ctx, key, 0, -1)
	if err != nil {
		t.Fatalf("ZRevRange 失败: %v", err)
	}
	assertMembers(t, "ZRevRange 全量", rev, []string{"d:20", "b:10", "a:10", "c:5"})

	// 下标区间与越界裁剪
	part, err := client.ZRange(ctx, key, 1, 2)
	if err != nil {
		t.Fatalf("ZRange 区间失败: %v", err)
	}
	assertMembers(t, "ZRange [1,2]", part, []string{"a:10", "b:10"})

	over, err := client.ZRange(ctx, key, -2, 100)
	if err != nil {
		t.Fatalf("ZRange 越界失败: %v", err)
	}
	assertMembers(t, "ZRange [-2,100]", over, []string{"b:10", "d:20"})

	empty, err := client.ZRange(ctx, key, 5, 10)
	if err != nil {
		t.Fatalf("ZRange 空区间失败: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("空区间应返回空切片, 实际: %v", zsetMembersToStrings(empty))
	}

	// 数量与分值
	if card, err := client.ZCard(ctx, key); err != nil || card != 4 {
		t.Fatalf("ZCard 不匹配: card=%d err=%v", card, err)
	}

	if score, ok, err := client.ZScore(ctx, key, "d"); err != nil || !ok || score != 20 {
		t.Fatalf("ZScore 不匹配: score=%g ok=%v err=%v", score, ok, err)
	}
	if _, ok, err := client.ZScore(ctx, key, "missing"); err != nil || ok {
		t.Fatalf("ZScore 不存在的成员应返回 ok=false: ok=%v err=%v", ok, err)
	}

	// 累加：已存在成员 + 新成员
	if score, err := client.ZIncrBy(ctx, key, "c", 15); err != nil || score != 20 {
		t.Fatalf("ZIncrBy 已存在成员失败: score=%g err=%v", score, err)
	}
	if score, err := client.ZIncrBy(ctx, key, "e", 1); err != nil || score != 1 {
		t.Fatalf("ZIncrBy 新成员失败: score=%g err=%v", score, err)
	}

	// 名次：c 与 d 同为 20，按字典序 c 在前
	if rank, ok, err := client.ZRank(ctx, key, "c"); err != nil || !ok || rank != 3 {
		t.Fatalf("ZRank 不匹配: rank=%d ok=%v err=%v", rank, ok, err)
	}
	if rank, ok, err := client.ZRevRank(ctx, key, "c"); err != nil || !ok || rank != 1 {
		t.Fatalf("ZRevRank 不匹配: rank=%d ok=%v err=%v", rank, ok, err)
	}
	if _, ok, err := client.ZRank(ctx, key, "missing"); err != nil || ok {
		t.Fatalf("ZRank 不存在的成员应返回 ok=false: ok=%v err=%v", ok, err)
	}

	// 更新既有成员分值后名次变化
	if err := client.ZAdd(ctx, key, map[string]float64{"c": 100}); err != nil {
		t.Fatalf("ZAdd 更新分值失败: %v", err)
	}
	if rank, ok, err := client.ZRevRank(ctx, key, "c"); err != nil || !ok || rank != 0 {
		t.Fatalf("更新分值后 ZRevRank 不匹配: rank=%d ok=%v err=%v", rank, ok, err)
	}

	// 删除
	if err := client.ZRem(ctx, key, "c", "e"); err != nil {
		t.Fatalf("ZRem 失败: %v", err)
	}
	if card, err := client.ZCard(ctx, key); err != nil || card != 3 {
		t.Fatalf("ZRem 后 ZCard 不匹配: card=%d err=%v", card, err)
	}

	// 删除全部后 key 消失
	if err := client.ZRem(ctx, key, "a", "b", "d"); err != nil {
		t.Fatalf("ZRem 全量失败: %v", err)
	}
	if exists, err := client.Exists(ctx, key); err != nil || exists {
		t.Fatalf("清空后 key 应不存在: exists=%v err=%v", exists, err)
	}
	if card, err := client.ZCard(ctx, key); err != nil || card != 0 {
		t.Fatalf("清空后 ZCard 应为 0: card=%d err=%v", card, err)
	}

	// 空参数删除直接返回成功
	if err := client.ZRem(ctx, key); err != nil {
		t.Fatalf("空参数 ZRem 不应报错: %v", err)
	}
}

// TestZSetMemory 验证内存后端的有序集合语义。
func TestZSetMemory(t *testing.T) {
	runZSetCases(t, newTestMemoryClient(t))
	t.Log("内存后端 ZSet 测试通过")
}

// TestZSetRedis 验证 Redis 后端的有序集合语义。
func TestZSetRedis(t *testing.T) {
	runZSetCases(t, newTestRedisClient(t))
	t.Log("Redis 后端 ZSet 测试通过")
}
