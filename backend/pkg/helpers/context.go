package helpers

import (
	"context"
	"time"
)

// CopyContext returns a new context that carries the Values from parent but has
// no deadline, no cancellation signal, and no Done channel.
//
// Use when spawning a background task that must outlive the original request
// (e.g. fire-and-forget activity log) but still needs Values like user_id or
// logger that were set on the request context.
func CopyContext(parent context.Context) context.Context {
	return &valueOnlyCtx{parent}
}

type valueOnlyCtx struct{ parent context.Context }

func (c *valueOnlyCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *valueOnlyCtx) Done() <-chan struct{}       { return nil }
func (c *valueOnlyCtx) Err() error                  { return nil }
func (c *valueOnlyCtx) Value(key any) any           { return c.parent.Value(key) }
