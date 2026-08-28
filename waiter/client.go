package waiter

import (
	"errors"
	"sync"
	"time"
)

// Result 等待结果。
type Result struct {
	Data  any
	Error error
}

// Client 等待器客户端。
type Client struct {
	waiters map[int64]*waiter
	lock    sync.Mutex
	stop    chan struct{}
	once    sync.Once
}

// New 创建等待器客户端，并启动后台清理线程。
func New() *Client {
	c := &Client{
		waiters: make(map[int64]*waiter),
		stop:    make(chan struct{}),
	}
	go c.cleanLoop()
	return c
}

// cleanLoop 后台清理线程：周期性扫描 waiters，销毁已过期的注册。
func (c *Client) cleanLoop() {
	ticker := time.NewTicker(cleanInterval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			c.purge(now)
		case <-c.stop:
			return
		}
	}
}

// purge 清理所有已过期的 waiter。
func (c *Client) purge(now time.Time) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for seq, w := range c.waiters {
		if w.isExpired(now) {
			delete(c.waiters, seq)
			close(w.ch)
		}
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
		ch:       make(chan Result, 1),
		deadline: time.Now().Add(timeout),
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
			// 通道已被关闭且未能投递结果（例如过期销毁）。
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

// Close 停止后台清理线程。可安全地多次调用。
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.stop)
	})
}
