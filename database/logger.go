package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	infralogger "github.com/minjieguo/infra/logger"
	"gorm.io/gorm/logger"
)

type Logger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
	debug         bool
	writer        infralogger.Logger
}

func newLogger(writer infralogger.Logger, debug bool) *Logger {
	level := logger.Warn
	if debug {
		level = logger.Info
	}
	return &Logger{
		LogLevel:      level,
		SlowThreshold: 200 * time.Millisecond,
		debug:         debug,
		writer:        writer,
	}
}

func (l *Logger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *Logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info && l.writer != nil {
		l.writer.Info(fmt.Sprintf(msg, data...))
	}
}

func (l *Logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn && l.writer != nil {
		l.writer.Warn(fmt.Sprintf(msg, data...))
	}
}

func (l *Logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error && l.writer != nil {
		l.writer.Error(fmt.Sprintf(msg, data...))
	}
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel == logger.Silent || l.writer == nil {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && l.LogLevel >= logger.Error && !errors.Is(err, logger.ErrRecordNotFound):
		l.writer.Error("gorm error",
			infralogger.Error(err),
			infralogger.String("sql", sql),
			infralogger.Int64("rows", rows),
			infralogger.Duration("elapsed", elapsed),
		)

	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		l.writer.Warn("gorm slow query",
			infralogger.String("sql", sql),
			infralogger.Int64("rows", rows),
			infralogger.Duration("elapsed", elapsed),
		)

	case l.debug && l.LogLevel >= logger.Info:
		l.writer.Info("gorm query",
			infralogger.String("sql", sql),
			infralogger.Int64("rows", rows),
			infralogger.Duration("elapsed", elapsed),
		)
	}
}
