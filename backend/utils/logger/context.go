package logger

import (
	"context"

	"go.uber.org/zap"
)

// traceIDKey context key，私有类型避免与其他包的 key 冲突
type traceIDKeyType struct{}

var traceIDKey = traceIDKeyType{}

// ContextWithTraceID 将 trace_id 写入 context；traceID 为空时原样返回
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil || traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext 从 context 中读取 trace_id，不存在返回空串
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// WithTraceID 返回携带 trace_id 字段的 zap.Logger。
// ctx 中无 trace_id 时返回全局 Logger 本身（如后台调度路径），不中断日志输出。
func WithTraceID(ctx context.Context) *zap.Logger {
	if Logger == nil {
		return zap.NewNop()
	}
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		return Logger
	}
	return Logger.With(zap.String("trace_id", traceID))
}
