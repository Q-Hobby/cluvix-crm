// Logger interface — Phase 1.3 of refactor-roadmap.
//
// Mục đích: tách abstract logging contract khỏi *zap.Logger cụ thể để:
//  1. Test có thể inject MemoryLogger capture log assertions.
//  2. Phase 3 DI có thể inject Logger qua Deps thay vì gọi global logger.L.
//
// API matching minimum subset của zap đang dùng trong codebase (L.Info/Warn/Error/Fatal).
// Field type giữ là zap.Field để không phải rewrite call sites — `zap.Error(err)`,
// `zap.String("k", v)` vẫn dùng được nguyên xi.

package logger

import "go.uber.org/zap"

// Logger là interface tối thiểu cho structured logging trong Cluvix BE.
// Wrap *zap.Logger qua Default(); test inject MemoryLogger.
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
}

// Default returns a Logger backed by package-global L (*zap.Logger).
// Returns nil-safe wrapper — nếu L chưa Init() thì các method no-op.
func Default() Logger {
	return &zapLogger{z: L}
}

// Wrap wraps an existing *zap.Logger as Logger (for cases where the caller
// has a non-global zap instance, e.g. scoped logger from .With()).
func Wrap(z *zap.Logger) Logger {
	return &zapLogger{z: z}
}

type zapLogger struct{ z *zap.Logger }

func (l *zapLogger) Debug(msg string, fields ...zap.Field) {
	if l.z == nil {
		return
	}
	l.z.Debug(msg, fields...)
}

func (l *zapLogger) Info(msg string, fields ...zap.Field) {
	if l.z == nil {
		return
	}
	l.z.Info(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...zap.Field) {
	if l.z == nil {
		return
	}
	l.z.Warn(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...zap.Field) {
	if l.z == nil {
		return
	}
	l.z.Error(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, fields ...zap.Field) {
	if l.z == nil {
		return
	}
	l.z.Fatal(msg, fields...)
}

func (l *zapLogger) With(fields ...zap.Field) Logger {
	if l.z == nil {
		return l
	}
	return &zapLogger{z: l.z.With(fields...)}
}
