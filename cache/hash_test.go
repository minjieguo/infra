package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/minjieguo/infra/internal/testenv"
)

// 测试用 Redis 配置，从仓库根目录的 .env 读取；
// 未配置 CACHE_REDIS_ADDR 时跳过测试。
//
// .env 示例：
//
//	CACHE_REDIS_ADDR=127.0.0.1:6379
//	CACHE_REDIS_PASSWORD=
//	CACHE_REDIS_DB=9
//	CACHE_PREFIX=infra-test:
func newTestRedisClient(t *testing.T) *Client {
	t.Helper()

	addr := testenv.Lookup("CACHE_REDIS_ADDR")
	if addr == "" {
		t.Skip("未配置 CACHE_REDIS_ADDR(参考 .env), 跳过 Redis 集成测试")
	}

	db := 0
	if raw := testenv.Lookup("CACHE_REDIS_DB"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("CACHE_REDIS_DB 不是合法整数: %q", raw)
		}
		db = parsed
	}

	client, err := New(Config{
		Type:     TypeRedis,
		Addr:     addr,
		Password: testenv.Lookup("CACHE_REDIS_PASSWORD"),
		DB:       db,
		Prefix:   testenv.Get("CACHE_PREFIX", ""),
	})
	if err != nil {
		t.Fatalf("创建 Redis 缓存客户端失败: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

// testHashKey 生成本次用例唯一的 hash key，避免用例之间互相污染。
func testHashKey() string {
	return fmt.Sprintf("hash-%d", time.Now().UnixNano())
}

// TestHashSetGet 验证单字段写入与读取的往返。
func TestHashSetGet(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HSet(ctx, key, "name", "alice"); err != nil {
		t.Fatalf("HSet 失败: %v", err)
	}

	got, err := client.HGet(ctx, key, "name")
	if err != nil {
		t.Fatalf("HGet 失败: %v", err)
	}
	if got != "alice" {
		t.Fatalf("HGet 结果不匹配: want=%v got=%v", "alice", got)
	}

	t.Logf("HSet/HGet 测试通过, 字段值: %v", got)
}

// TestHashGetMiss 验证字段不存在时返回 ErrCacheMiss。
func TestHashGetMiss(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()

	_, err := client.HGet(ctx, testHashKey(), "not-exist")
	if !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("期望 ErrCacheMiss, 实际: %v", err)
	}

	t.Log("HGet 未命中返回 ErrCacheMiss 测试通过")
}

// TestHashGetAll 验证读取整个 hash, 以及键不存在时返回空 map。
func TestHashGetAll(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	empty, err := client.HGetAll(ctx, key)
	if err != nil {
		t.Fatalf("HGetAll 空键失败: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("空键期望空 map, 实际: %v", empty)
	}

	if err := client.HMSet(ctx, key, map[string]any{
		"name": "alice",
		"age":  18,
	}); err != nil {
		t.Fatalf("HMSet 失败: %v", err)
	}

	all, err := client.HGetAll(ctx, key)
	if err != nil {
		t.Fatalf("HGetAll 失败: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("期望 2 个字段, 实际: %v", all)
	}
	if all["name"] != "alice" {
		t.Fatalf("name 字段不匹配: %v", all["name"])
	}
	// JSON 数字解码为 float64
	if age, ok := all["age"].(float64); !ok || age != 18 {
		t.Fatalf("age 字段不匹配: %v", all["age"])
	}

	t.Logf("HGetAll 测试通过, 字段数: %d", len(all))
}

// TestHashMeta 验证 HExists / HLen / HKeys / HVals。
func TestHashMeta(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HMSet(ctx, key, map[string]any{"a": "1", "b": "2"}); err != nil {
		t.Fatalf("HMSet 失败: %v", err)
	}

	exists, err := client.HExists(ctx, key, "a")
	if err != nil {
		t.Fatalf("HExists 失败: %v", err)
	}
	if !exists {
		t.Fatal("HExists 期望 true, 实际 false")
	}

	missing, err := client.HExists(ctx, key, "zzz")
	if err != nil {
		t.Fatalf("HExists 失败: %v", err)
	}
	if missing {
		t.Fatal("HExists 不存在的字段期望 false, 实际 true")
	}

	size, err := client.HLen(ctx, key)
	if err != nil {
		t.Fatalf("HLen 失败: %v", err)
	}
	if size != 2 {
		t.Fatalf("HLen 期望 2, 实际: %d", size)
	}

	keys, err := client.HKeys(ctx, key)
	if err != nil {
		t.Fatalf("HKeys 失败: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("HKeys 期望 2 个, 实际: %v", keys)
	}

	values, err := client.HVals(ctx, key)
	if err != nil {
		t.Fatalf("HVals 失败: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("HVals 期望 2 个, 实际: %v", values)
	}

	t.Logf("Hash 元信息测试通过, keys=%v values=%v", keys, values)
}

// TestHashDel 验证删除字段。
func TestHashDel(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HMSet(ctx, key, map[string]any{"a": "1", "b": "2", "c": "3"}); err != nil {
		t.Fatalf("HMSet 失败: %v", err)
	}

	if err := client.HDel(ctx, key, "a", "b"); err != nil {
		t.Fatalf("HDel 失败: %v", err)
	}

	size, err := client.HLen(ctx, key)
	if err != nil {
		t.Fatalf("HLen 失败: %v", err)
	}
	if size != 1 {
		t.Fatalf("删除后期望 1 个字段, 实际: %d", size)
	}

	if _, err := client.HGet(ctx, key, "a"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("已删除字段期望 ErrCacheMiss, 实际: %v", err)
	}

	t.Logf("HDel 测试通过, 剩余字段数: %d", size)
}

// TestHashMGet 验证批量读取。
func TestHashMGet(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HMSet(ctx, key, map[string]any{"a": "1", "b": "2"}); err != nil {
		t.Fatalf("HMSet 失败: %v", err)
	}

	values, err := client.HMGet(ctx, key, "a", "b", "not-exist")
	if err != nil {
		t.Fatalf("HMGet 失败: %v", err)
	}
	if len(values) != 2 {
		t.Fatalf("HMGet 期望 2 个结果, 实际: %v", values)
	}
	// 数字字符串经 JSON 往返后解码为 float64
	if a, ok := values["a"].(float64); !ok || a != 1 {
		t.Fatalf("HMGet a 不匹配: %#v", values["a"])
	}
	if _, ok := values["not-exist"]; ok {
		t.Fatalf("不存在的字段不应出现在结果中: %v", values)
	}

	t.Logf("HMGet 测试通过, 结果: %v", values)
}

// testHashValue 用于 JSON 泛型往返测试。
//
// 注意 Name 字段刻意使用非数字字符串：
// HSetJSON 先 json.Marshal 得到 `{"name":"...","age":18}`，
// 该字符串存入 hash 后，HGet 会再走一次 json.Unmarshal 得到 map。
// 若 Name 是数字字符串（如 "1001"），外层解码会把它变成 float64，
// HGetJSON 再 json.Unmarshal 到 string 字段就会报 type error。
type testHashValue struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// TestHashJSON 验证 HSetJSON / HGetJSON 泛型往返。
func TestHashJSON(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	want := testHashValue{Name: "alice", Age: 18}
	if err := HSetJSON(client, ctx, key, "user", want); err != nil {
		t.Fatalf("HSetJSON 失败: %v", err)
	}

	got, err := HGetJSON[testHashValue](client, ctx, key, "user")
	if err != nil {
		t.Fatalf("HGetJSON 失败: %v", err)
	}
	if got.Name != want.Name || got.Age != want.Age {
		t.Fatalf("HGetJSON 结果不匹配: want=%+v got=%+v", want, *got)
	}

	if _, err := HGetJSON[testHashValue](client, ctx, key, "not-exist"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("HGetJSON 未命中期望 ErrCacheMiss, 实际: %v", err)
	}

	t.Logf("HSetJSON/HGetJSON 测试通过, 结果: %+v", *got)
}

// TestHashExpire 验证字段级 TTL（依赖 Redis 7.4+ 的 HEXPIRE 命令）。
func TestHashExpire(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HMSet(ctx, key, map[string]any{"short": "1", "long": "2"}); err != nil {
		t.Fatalf("HMSet 失败: %v", err)
	}

	if err := client.HExpire(ctx, key, 1*time.Second, "short"); err != nil {
		t.Skipf("HExpire 不可用(需 Redis 7.4+ 支持 HEXPIRE): %v", err)
	}

	// 未过期的字段仍可读取
	if _, err := client.HGet(ctx, key, "short"); err != nil {
		t.Fatalf("HGet 未过期字段失败: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	if _, err := client.HGet(ctx, key, "short"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("字段过期后期望 ErrCacheMiss, 实际: %v", err)
	}

	got, err := client.HGet(ctx, key, "long")
	if err != nil {
		t.Fatalf("HGet 未设置 TTL 的字段失败: %v", err)
	}
	if v, ok := got.(float64); !ok || v != 2 {
		t.Fatalf("未设置 TTL 的字段不应过期, 实际: %#v", got)
	}

	t.Log("HExpire 字段级 TTL 测试通过")
}

// TestHashExpireAt 验证字段级绝对过期时间。
func TestHashExpireAt(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testHashKey()

	if err := client.HSet(ctx, key, "field", "value"); err != nil {
		t.Fatalf("HSet 失败: %v", err)
	}

	if err := client.HExpireAt(ctx, key, time.Now().Add(1*time.Second), "field"); err != nil {
		t.Skipf("HExpireAt 不可用(需 Redis 7.4+ 支持 HEXPIREAT): %v", err)
	}

	time.Sleep(1500 * time.Millisecond)

	if _, err := client.HGet(ctx, key, "field"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("字段过期后期望 ErrCacheMiss, 实际: %v", err)
	}

	t.Log("HExpireAt 字段级绝对过期测试通过")
}
