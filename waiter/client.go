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

	defer func() {
		c.lock.Lock()
		delete(c.waiters, seq)
		c.lock.Unlock()
	}()

	select {
	case result, ok := <-w.ch:
		if !ok {
			// 通道已被关闭且未能投递结果（例如被 Cancel）。
			return nil, errors.New("channel closed")
		}
		return &result, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}

// Resolve 写入结果；如果没有有效的等待者则返回 false。
func (c *Client) Resolve(seq int64, value Result) bool {
	c.lock.Lock()
	w, ok := c.waiters[seq]
	if ok {
		delete(c.waiters, seq)
	}
	c.lock.Unlock()

	if !ok {
		return false
	}

	select {
	case w.ch <- value:
	default:
	}
	close(w.ch)
	return true
}

// Close 关闭指定 seq 的等待，关闭通道以唤醒等待者。
// 如果该 seq 未注册，则不做任何事。
func (c *Client) Close(seq int64) {
	c.lock.Lock()
	w, ok := c.waiters[seq]
	if ok {
		delete(c.waiters, seq)
	}
	c.lock.Unlock()

	if ok {
		close(w.ch)
	}
}
