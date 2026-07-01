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

var (
	channels map[int64]chan Result = make(map[int64]chan Result)
	lock     sync.Mutex
)

// Register 注册等待通道，返回监听通道
func Register(seq int64) {
	lock.Lock()
	defer lock.Unlock()

	if _, exists := channels[seq]; exists {
		return
	}

	ch := make(chan Result, 1)
	channels[seq] = ch
}

func Await(seq int64, timeout time.Duration) (*Result, error) {

	if seq == 0 {
		return nil, errors.New("seq can not be 0")
	}

	lock.Lock()
	ch, exists := channels[seq]
	lock.Unlock()

	if !exists {
		return nil, errors.New("channel not registered")
	}

	defer func() {
		lock.Lock()
		delete(channels, seq)
		lock.Unlock()
	}()

	select {
	case result := <-ch:
		return &result, nil
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}

// Resolve 写入结果；如果没有等待者则返回 false
func Resolve(seq int64, value Result) bool {
	lock.Lock()
	ch, ok := channels[seq]
	if ok {
		delete(channels, seq)
	}
	lock.Unlock()

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
