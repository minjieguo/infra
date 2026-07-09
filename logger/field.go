package logger

import (
	"time"

	"go.uber.org/zap"
)

type Field struct {
	field zap.Field
}

func Binary(key string, val []byte) Field {
	return Field{field: zap.Binary(key, val)}
}

func String(key, val string) Field {
	return Field{field: zap.String(key, val)}
}

func Bool(key string, val bool) Field {
	return Field{field: zap.Bool(key, val)}
}

func Int(key string, val int) Field {
	return Field{field: zap.Int(key, val)}
}

func Int64(key string, val int64) Field {
	return Field{field: zap.Int64(key, val)}
}

func Duration(key string, val time.Duration) Field {
	return Field{field: zap.Duration(key, val)}
}

func Error(err error) Field {
	return Field{field: zap.Error(err)}
}

func Any(key string, val any) Field {
	return Field{field: zap.Any(key, val)}
}

func toZapFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		zapFields = append(zapFields, field.field)
	}
	return zapFields
}
