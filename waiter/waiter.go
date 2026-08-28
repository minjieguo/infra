package waiter

// Result 等待结果。
type Result struct {
	Data  any
	Error error
}

// waiter 一次等待注册。
type waiter struct {
	ch chan Result
}
