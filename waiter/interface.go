package waiter

import "time"

// Waiter 等待器接口。
type Waiter interface {
	Register(seq int64)
	Await(seq int64, timeout time.Duration) (*Result, error)
	Resolve(seq int64, value Result) bool
}
