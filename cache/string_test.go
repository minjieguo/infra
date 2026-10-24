package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// testStringKey 生成本次用例唯一的字符串 key，避免用例之间互相污染。
func testStringKey() string {
	return fmt.Sprintf("string-%d", time.Now().UnixNano())
}

// runStringCounterCases 验证计数、存在性、过期、批量删除在各后端的一致行为。
func runStringCounterCases(t *testing.T, client *Client) {
	t.Helper()

	ctx := context.Background()
	counterKey := testStringKey()

	if got, err := client.IncrBy(ctx, counterKey, 5); err != nil || got != 5 {
		t.Fatalf("IncrBy 首次不匹配: got=%d err=%v", got, err)
	}
	if got, err := client.IncrBy(ctx, counterKey, 3); err != nil || got != 8 {
		t.Fatalf("IncrBy 累加不匹配: got=%d err=%v", got, err)
	}
	if got, err := client.DecrBy(ctx, counterKey, 10); err != nil || got != -2 {
		t.Fatalf("DecrBy 跨零不匹配: got=%d err=%v", got, err)
	}

	if exists, err := client.Exists(ctx, counterKey); err != nil || !exists {
		t.Fatalf("计数 key 应存在: exists=%v err=%v", exists, err)
	}

	// 计数 key 没有设置过期时间
	if ttl, err := client.TTL(ctx, counterKey); err != nil || ttl != TTLNoExpire {
		t.Fatalf("计数 key TTL 应为 TTLNoExpire: ttl=%v err=%v", ttl, err)
	}

	// 其他结构类型的 key 也算存在
	hashKey := testStringKey()
	if err := client.HSet(ctx, hashKey, "name", "alice"); err != nil {
		t.Fatalf("HSet 失败: %v", err)
	}
	if exists, err := client.Exists(ctx, hashKey); err != nil || !exists {
		t.Fatalf("hash key 应存在: exists=%v err=%v", exists, err)
	}

	zsetKey := testStringKey()
	if err := client.ZAdd(ctx, zsetKey, map[string]float64{"a": 1}); err != nil {
		t.Fatalf("ZAdd 失败: %v", err)
	}
	if exists, err := client.Exists(ctx, zsetKey); err != nil || !exists {
		t.Fatalf("zset key 应存在: exists=%v err=%v", exists, err)
	}

	// 不存在的 key
	if exists, err := client.Exists(ctx, testStringKey()); err != nil || exists {
		t.Fatalf("不存在的 key 应为 false: exists=%v err=%v", exists, err)
	}
	if _, err := client.TTL(ctx, testStringKey()); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("不存在的 key TTL 应返回 ErrCacheMiss, 实际: %v", err)
	}

	// Set 写入的字符串 key 的过期与 TTL
	setKey := testStringKey()
	if err := client.Set(ctx, setKey, "value", time.Minute); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	ttl, err := client.TTL(ctx, setKey)
	if err != nil {
		t.Fatalf("TTL 失败: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("字符串 key TTL 应在 (0, 1m] 区间: ttl=%v", ttl)
	}

	// 批量删除
	if err := client.DeleteMany(ctx, counterKey, hashKey, zsetKey, setKey); err != nil {
		t.Fatalf("DeleteMany 失败: %v", err)
	}
	for _, key := range []string{counterKey, hashKey, zsetKey, setKey} {
		if exists, err := client.Exists(ctx, key); err != nil || exists {
			t.Fatalf("DeleteMany 后 key 仍存在: key=%s exists=%v err=%v", key, exists, err)
		}
	}

	// 空参数删除直接返回成功
	if err := client.DeleteMany(ctx); err != nil {
		t.Fatalf("空参数 DeleteMany 不应报错: %v", err)
	}
}

// TestStringCounterMemory 验证内存后端的计数与批量删除语义。
func TestStringCounterMemory(t *testing.T) {
	client := newTestMemoryClient(t)
	runStringCounterCases(t, client)
	t.Log("内存后端 String 扩展测试通过")
}

// TestStringCounterRedis 验证 Redis 后端的计数与批量删除语义。
func TestStringCounterRedis(t *testing.T) {
	client := newTestRedisClient(t)
	runStringCounterCases(t, client)
	t.Log("Redis 后端 String 扩展测试通过")
}

// TestStringExpireMemory 验证内存后端的 key 级过期行为。
//
// 内存后端的过期时间基于 memoryExpireNow，用例中替换该函数以模拟时间流逝。
func TestStringExpireMemory(t *testing.T) {
	client := newTestMemoryClient(t)
	ctx := context.Background()

	base := time.Now()
	original := memoryExpireNow
	memoryExpireNow = func() time.Time { return base }
	t.Cleanup(func() {
		memoryExpireNow = original
	})

	zsetKey := testStringKey()
	if err := client.ZAdd(ctx, zsetKey, map[string]float64{"a": 1}); err != nil {
		t.Fatalf("ZAdd 失败: %v", err)
	}
	if err := client.Expire(ctx, zsetKey, time.Minute); err != nil {
		t.Fatalf("Expire 失败: %v", err)
	}

	ttl, err := client.TTL(ctx, zsetKey)
	if err != nil {
		t.Fatalf("TTL 失败: %v", err)
	}
	if ttl != time.Minute {
		t.Fatalf("TTL 不匹配: want=1m got=%v", ttl)
	}

	// 未到期时依然可访问
	memoryExpireNow = func() time.Time { return base.Add(30 * time.Second) }
	if exists, err := client.Exists(ctx, zsetKey); err != nil || !exists {
		t.Fatalf("未到期 key 应存在: exists=%v err=%v", exists, err)
	}

	// 到期后 key 被惰性淘汰
	memoryExpireNow = func() time.Time { return base.Add(2 * time.Minute) }
	if exists, err := client.Exists(ctx, zsetKey); err != nil || exists {
		t.Fatalf("到期 key 应不存在: exists=%v err=%v", exists, err)
	}
	if card, err := client.ZCard(ctx, zsetKey); err != nil || card != 0 {
		t.Fatalf("到期后 ZCard 应为 0: card=%d err=%v", card, err)
	}

	t.Log("内存后端过期测试通过")
}

// TestStringIncrByConcurrent 验证内存后端计数在并发调用下不丢更新。
func TestStringIncrByConcurrent(t *testing.T) {
	client := newTestMemoryClient(t)
	ctx := context.Background()
	key := testStringKey()

	const (
		goroutines = 20
		perRoutine = 100
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perRoutine; j++ {
				if _, err := client.IncrBy(ctx, key, 1); err != nil {
					t.Errorf("IncrBy 失败: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	got, err := client.IncrBy(ctx, key, 0)
	if err != nil {
		t.Fatalf("读取计数失败: %v", err)
	}
	if want := int64(goroutines * perRoutine); got != want {
		t.Fatalf("并发累加结果不匹配: want=%d got=%d", want, got)
	}

	t.Log("并发计数测试通过")
}
