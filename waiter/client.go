package waiter

import (
	"errors"
	"sync"
	"time"
)

// Client 等待器客户端。
type Client struct {
	waiters map[int64]*waiter
	lock    sync.Mutex
}

// New 创建等待器客户端。
func New() *Client {
	return &Client{
		waiters: make(map[int64]*waiter),
	}
}

// Register 注册等待通道。
func (c *Client) Register(seq int64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, exists := c.waiters[seq]; exists {
		return
	}

	c.waiters[seq] = &waiter{
		ch: make(chan Result, 1),
	}
}

// Await 阻塞等待结果，超时或注册失效则返回错误。
func (c *Client) Await(seq int64, timeout time.Duration) (*Result, error) {
	if seq == 0 {
		return nil, errors.New("seq can not be 0")
	}

	c.lock.Lock()
	w, exists := c.waiters[seq]
	c.lock.Unlock()

	if !exists {
		return nil, errors.New("channel not registered")
	}

	defer c.removeWaiter(seq, w)

	var result Result
	var ok bool
	// 优先读取已投递的结果，即使 timeout 为零也不会丢失。
	select {
	case result, ok = <-w.ch:
	default:
		select {
		case result, ok = <-w.ch:
		case <-time.After(timeout):
			return nil, errors.New("timeout")
		}
	}
	if !ok {
		return nil, errors.New("channel closed")
	}
	return &result, nil
}

// removeWaiter 仅清理 Await 取得的注册，避免误删同序号的新注册。
func (c *Client) removeWaiter(seq int64, w *waiter) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.waiters[seq] == w {
		delete(c.waiters, seq)
	}
}

// Resolve 写入首次结果；未注册或已投递时返回 false。
func (c *Client) Resolve(seq int64, value Result) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	w, ok := c.waiters[seq]
	if !ok || w.resolved {
		return false
	}

	// 每次注册只写入一次，容量为 1 的通道保证此处不会阻塞。
	w.resolved = true
	w.ch <- value
	return true
}

// Close 关闭指定 seq 的等待，关闭通道以唤醒等待者。
// 如果该 seq 未注册，则不做任何事。
func (c *Client) Close(seq int64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if w, ok := c.waiters[seq]; ok {
		delete(c.waiters, seq)
		close(w.ch)
	}
}
