package logger

import (
	"go.uber.org/zap"
)

// Logger 全局 zap 日志实例
var Logger *zap.Logger

// InitLogger 初始化 zap 日志
func InitLogger(level string) error {
	cfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	l, err := cfg.Build()
	if err != nil {
		return err
	}

	Logger = l
	return nil
}

// Sync 刷出日志缓冲
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
