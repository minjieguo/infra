package queue

import "go.uber.org/zap"

// defaultLogger 默认空日志实现,当未传入 Logger 时使用
type defaultLogger struct{}

func (defaultLogger) Debug(string, ...zap.Field) {}
func (defaultLogger) Info(string, ...zap.Field)  {}
func (defaultLogger) Warn(string, ...zap.Field)  {}
func (defaultLogger) Error(string, ...zap.Field) {}
