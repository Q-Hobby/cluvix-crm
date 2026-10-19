package helpers

import (
	"context"

	"erpCluvix/pkg/logger"

	"go.uber.org/zap"
)

// RunBackground spawns fn in a goroutine with panic recovery and structured logging.
// Use instead of bare `go func()` whenever the goroutine is fire-and-forget.
func RunBackground(ctx context.Context, name string, fn func(context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L.Error("goroutine panic recovered",
					zap.String("name", name),
					zap.Any("panic", r),
				)
			}
		}()
		fn(ctx)
	}()
}
