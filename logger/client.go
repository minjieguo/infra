package logger

import (
	"fmt"
	"os"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 日志配置。
type Config struct {
	Stdout       bool          // 输出到控制台
	Path         string        // 日志路径: 默认  logs
	MaxAge       time.Duration // 保留日志的时间 默认90天
	RotationTime time.Duration // 切割文件的规则 默认每天
}

// Client 日志客户端。
type Client struct {
	logger *zap.Logger
}

func New(cfg Config) (*Client, error) {
	if cfg.Path == "" {
		cfg.Path = "logs"
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 90 * 24 * time.Hour //保留90天
	}
	if cfg.RotationTime == 0 {
		cfg.RotationTime = 24 * time.Hour //每天切割
	}

	// 日志文件
	err := os.MkdirAll(cfg.Path, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 日志输出
	writer, err := rotatelogs.New(
		cfg.Path+"/%Y-%m-%d.log",                      // 每天一个文件
		rotatelogs.WithMaxAge(cfg.MaxAge),             //
		rotatelogs.WithRotationTime(cfg.RotationTime), //
	)
	if err != nil {
		return nil, fmt.Errorf("创建日志输出失败: %w", err)
	}

	// 编码器配置（文本格式）
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05"))
	}

	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	tree := make([]zapcore.Core, 0)
	tree = append(tree, zapcore.NewCore(encoder, zapcore.AddSync(writer), zap.InfoLevel))
	//输出控制台
	if cfg.Stdout {
		tree = append(tree, zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zap.InfoLevel))
	}
	core := zapcore.NewTee(tree...)

	return &Client{logger: zap.New(core, zap.AddCaller())}, nil
}

func (c *Client) Info(msg string, fields ...zap.Field) {
	c.zap().Info(msg, fields...)
}

func (c *Client) Warn(msg string, fields ...zap.Field) {
	c.zap().Warn(msg, fields...)
}

func (c *Client) Error(msg string, fields ...zap.Field) {
	c.zap().Error(msg, fields...)
}

func (c *Client) Sync() error {
	return c.zap().Sync()
}

func (c *Client) Sugar() *zap.SugaredLogger {
	return c.zap().Sugar()
}

func (c *Client) zap() *zap.Logger {
	if c == nil || c.logger == nil {
		return zap.NewNop()
	}
	return c.logger
}
