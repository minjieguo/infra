package mqtt

// Interface logger interface
type Logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

type defaultLogger struct{}

func (defaultLogger) Info(string, ...any)  {}
func (defaultLogger) Warn(string, ...any)  {}
func (defaultLogger) Error(string, ...any) {}
