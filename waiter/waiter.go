package waiter

import "time"

const (
	// timeout 注册的默认过期时间。
	timeout = 10 * time.Minute

	// cleanInterval 后台清理线程的扫描间隔。
	cleanInterval = time.Minute
)

// waiter 一次等待注册。
type waiter struct {
	ch       chan Result
	deadline time.Time // 过期时间（绝对时间）
}

// isExpired 判断是否已过期。
func (w *waiter) isExpired(now time.Time) bool {
	return !now.Before(w.deadline)
}
