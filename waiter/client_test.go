package waiter

import (
	"testing"
	"time"
)

func TestAwaitThenResolve(t *testing.T) {
	c := New()

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
	if _, err := c.Await(42, time.Second); err == nil {
		t.Fatal("expected error for unregistered seq")
	}
}

func TestAwaitZeroSeq(t *testing.T) {
	c := New()
	if _, err := c.Await(0, time.Second); err == nil || err.Error() != "seq can not be 0" {
		t.Fatalf("expected seq zero error, got %v", err)
	}
}

func TestCancelWakesAwait(t *testing.T) {
	c := New()

	c.Register(1)

	errCh := make(chan error, 1)
	go func() {
		_, err := c.Await(1, time.Second)
		errCh <- err
	}()

	time.Sleep(10 * time.Millisecond)

	c.Close(1)

	select {
	case err := <-errCh:
		if err == nil || err.Error() != "channel closed" {
			t.Fatalf("expected 'channel closed' error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Await to return after Cancel")
	}

	// Cancel 后注册应被清除。
	c.lock.Lock()
	_, exists := c.waiters[1]
	c.lock.Unlock()
	if exists {
		t.Fatalf("expected registration removed after Cancel")
	}
}

func TestCancelUnregistered(t *testing.T) {
	c := New()
	c.Close(42) // 不应 panic
}

func TestResolveAfterCancel(t *testing.T) {
	c := New()

	c.Register(1)
	c.Close(1)

	if c.Resolve(1, Result{Data: "x"}) {
		t.Fatalf("expected Resolve to return false after Cancel")
	}
}
