package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// testQueueKey 生成本次用例唯一的队列 key，避免用例之间互相污染。
func testQueueKey() string {
	return fmt.Sprintf("queue-%d", time.Now().UnixNano())
}

// TestQueuePushPop 验证 LPUSH + BRPOP 的先进先出顺序。
func TestQueuePushPop(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()
	key := testQueueKey()

	for _, value := range []string{"a", "b", "c"} {
		if err := client.Push(ctx, key, value); err != nil {
			t.Fatalf("Push %s 失败: %v", value, err)
		}
	}

	for _, want := range []string{"a", "b", "c"} {
		got, err := client.Pop(ctx, key)
		if err != nil {
			t.Fatalf("Pop 失败: %v", err)
		}
		if got != want {
			t.Fatalf("Pop 顺序不匹配: want=%s got=%s", want, got)
		}
	}

	t.Log("Push/Pop 先进先出测试通过")
}

// TestQueuePopBlocks 验证空队列时 Pop 阻塞，直到有元素入队。
func TestQueuePopBlocks(t *testing.T) {
	client := newTestRedisClient(t)
	key := testQueueKey()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := client.Pop(ctx, key)
		done <- result{value: value, err: err}
	}()

	select {
	case got := <-done:
		t.Fatalf("空队列 Pop 不应立即返回: %+v", got)
	case <-time.After(200 * time.Millisecond):
	}

	if err := client.Push(ctx, key, "hello"); err != nil {
		t.Fatalf("Push 失败: %v", err)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("Pop 失败: %v", got.err)
		}
		if got.value != "hello" {
			t.Fatalf("Pop 结果不匹配: want=hello got=%s", got.value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Push 之后 Pop 仍未返回")
	}

	t.Log("Pop 阻塞唤醒测试通过")
}

// TestQueuePopContextCancel 验证 ctx 结束会中断阻塞的 Pop。
func TestQueuePopContextCancel(t *testing.T) {
	client := newTestRedisClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Pop(ctx, testQueueKey())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("期望 context.DeadlineExceeded, 实际: %v", err)
	}

	t.Log("Pop ctx 超时中断测试通过")
}

// TestQueueMemory 验证内存后端的 Push/Pop/阻塞/关闭语义。
func TestQueueMemory(t *testing.T) {
	client, err := New(Config{Type: TypeMemory})
	if err != nil {
		t.Fatalf("创建内存缓存客户端失败: %v", err)
	}

	ctx := context.Background()
	key := testQueueKey()

	for _, value := range []string{"a", "b"} {
		if err := client.Push(ctx, key, value); err != nil {
			t.Fatalf("Push %s 失败: %v", value, err)
		}
	}
	for _, want := range []string{"a", "b"} {
		got, err := client.Pop(ctx, key)
		if err != nil {
			t.Fatalf("Pop 失败: %v", err)
		}
		if got != want {
			t.Fatalf("Pop 顺序不匹配: want=%s got=%s", want, got)
		}
	}

	// 空队列阻塞，写入后被唤醒
	done := make(chan string, 1)
	go func() {
		value, popErr := client.Pop(ctx, key)
		if popErr != nil {
			value = "err:" + popErr.Error()
		}
		done <- value
	}()
	time.Sleep(100 * time.Millisecond)
	if err := client.Push(ctx, key, "wake"); err != nil {
		t.Fatalf("Push 失败: %v", err)
	}
	select {
	case got := <-done:
		if got != "wake" {
			t.Fatalf("阻塞唤醒结果不匹配: got=%s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Push 之后 Pop 仍未返回")
	}

	// Close 后阻塞中的 Pop 返回 ErrQueueClosed
	closedDone := make(chan error, 1)
	go func() {
		_, popErr := client.Pop(ctx, testQueueKey())
		closedDone <- popErr
	}()
	time.Sleep(100 * time.Millisecond)
	if err := client.Close(); err != nil {
		t.Fatalf("Close 失败: %v", err)
	}
	select {
	case popErr := <-closedDone:
		if !errors.Is(popErr, ErrQueueClosed) {
			t.Fatalf("期望 ErrQueueClosed, 实际: %v", popErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close 之后阻塞的 Pop 仍未返回")
	}

	t.Log("内存队列测试通过")
}
