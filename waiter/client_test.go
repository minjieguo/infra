package waiter

import (
	"errors"
	"sync"
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

func TestResolveBeforeAwait(t *testing.T) {
	c := New()
	want := Result{Data: "first", Error: errors.New("设备离线")}

	c.Register(1)
	if !c.Resolve(1, want) {
		t.Fatal("expected first Resolve to succeed")
	}
	c.Register(1)
	if c.Resolve(1, Result{Data: "second"}) {
		t.Fatal("expected duplicate Resolve to fail")
	}
	got, err := c.Await(1, 0)
	if err != nil || got == nil || *got != want {
		t.Fatalf("Await got (%+v, %v), want (%+v, nil)", got, err, want)
	}

	c.Register(1)
	if !c.Resolve(1, Result{Data: "new"}) {
		t.Fatal("expected Resolve to succeed after re-registering")
	}
	got, err = c.Await(1, time.Second)
	if err != nil || got == nil || got.Data != "new" {
		t.Fatalf("unexpected result after re-registering: (%+v, %v)", got, err)
	}
}

func TestResolveConcurrentWithClose(t *testing.T) {
	c := New()
	for i := 0; i < 100; i++ {
		c.Register(1)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			c.Resolve(1, Result{Data: "hello"})
		}()
		go func() {
			defer wg.Done()
			<-start
			c.Close(1)
		}()
		close(start)
		wg.Wait()
		if c.Resolve(1, Result{}) {
			t.Fatal("expected registration removed after Close")
		}
	}
}

func TestAwaitCleanupPreservesNewRegistration(t *testing.T) {
	c := New()
	c.Register(1)
	old := c.waiters[1]
	c.Close(1)
	c.Register(1)

	// 模拟旧 Await 在同一序号重新注册后才执行延迟清理。
	c.removeWaiter(1, old)
	if !c.Resolve(1, Result{Data: "new"}) {
		t.Fatal("old Await cleanup removed the new registration")
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
