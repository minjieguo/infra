package waiter

import (
	"testing"
	"time"
)

func TestRegisterExpiresAutomatically(t *testing.T) {
	c := New()
	defer c.Close()

	c.Register(1)

	// 手动将 deadline 置为过去，模拟已过期。
	c.lock.Lock()
	c.waiters[1].deadline = time.Now().Add(-time.Second)
	c.lock.Unlock()

	// 触发一次清理，注册应被销毁。
	c.purge(time.Now())

	c.lock.Lock()
	_, exists := c.waiters[1]
	c.lock.Unlock()

	if exists {
		t.Fatalf("expected expired registration to be removed")
	}

	if _, err := c.Await(1, time.Second); err == nil {
		t.Fatalf("expected error for expired registration")
	}
	if c.Resolve(1, Result{Data: "x"}) {
		t.Fatalf("expected Resolve to return false for expired registration")
	}
}

func TestAwaitThenResolve(t *testing.T) {
	c := New()
	defer c.Close()

	c.Register(1)

	resultCh := make(chan *Result, 1)
	errCh := make(chan error, 1)
	go func() {
		res, err := c.Await(1, time.Second)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- res
	}()

	time.Sleep(10 * time.Millisecond)

	if !c.Resolve(1, Result{Data: "hello"}) {
		t.Fatalf("expected Resolve to succeed")
	}

	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case res := <-resultCh:
		if res.Data != "hello" {
			t.Fatalf("unexpected data: %v", res.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Await result")
	}
}

func TestAwaitTimeout(t *testing.T) {
	c := New()
	defer c.Close()

	c.Register(1)
	_, err := c.Await(1, 10*time.Millisecond)
	if err == nil || err.Error() != "timeout" {
		t.Fatalf("expected timeout error, got %v", err)
	}

	c.lock.Lock()
	_, exists := c.waiters[1]
	c.lock.Unlock()
	if exists {
		t.Fatalf("expected registration removed after timeout")
	}
}

func TestAwaitUnregistered(t *testing.T) {
	c := New()
	defer c.Close()
	if _, err := c.Await(42, time.Second); err == nil {
		t.Fatal("expected error for unregistered seq")
	}
}

func TestAwaitZeroSeq(t *testing.T) {
	c := New()
	defer c.Close()
	if _, err := c.Await(0, time.Second); err == nil || err.Error() != "seq can not be 0" {
		t.Fatalf("expected seq zero error, got %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	c := New()
	c.Close()
	c.Close() // 不应 panic
}
