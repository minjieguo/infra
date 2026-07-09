package mqtt

import "go.uber.org/zap"

// // Interface logger interface
// type Logger interface {
// 	Info(string, ...any)
// 	Warn(string, ...any)
// 	Error(string, ...any)
// }

type defaultLogger struct{}

func (defaultLogger) Debug(string, ...zap.Field) {}
func (defaultLogger) Info(string, ...zap.Field)  {}
func (defaultLogger) Warn(string, ...zap.Field)  {}
func (defaultLogger) Error(string, ...zap.Field) {}
