package waiter

import (
	"errors"
	"sync"
	"time"
)

type Result struct {
	Data  any
	Error error
}

// Client 等待器客户端。
type Client struct {
	channels map[int64]chan Result
	lock     sync.Mutex
}

func New() *Client {
	return &Client{
		channels: make(map[int64]chan Result),
	}
}

// Register 注册等待通道，返回监听通道
func (c *Client) Register(seq int64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, exists := c.channels[seq]; exists {
		return
	}

	ch := make(chan Result, 1)
	c.channels[seq] = ch
}

func (c *Client) Await(seq int64, timeout time.Duration) (*Result, error) {

	if seq == 0 {
		return nil, errors.New("seq can not be 0")
	}

	c.lock.Lock()
	ch, exists := c.channels[seq]
	c.lock.Unlock()

	if !exists {
		return nil, errors.New("channel not registered")
	}

	defer func() {
		c.lock.Lock()
		delete(c.channels, seq)
		c.lock.Unlock()
	}()

	select {
	case result := <-ch:
		return &result, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}

// Resolve 写入结果；如果没有等待者则返回 false
func (c *Client) Resolve(seq int64, value Result) bool {
	c.lock.Lock()
	ch, ok := c.channels[seq]
	if ok {
		delete(c.channels, seq)
	}
	c.lock.Unlock()

	if !ok {
		return false
	}

	select {
	case ch <- value:
	default:
	}
	close(ch)
	return true
}
