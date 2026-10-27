// Package syncutil provides panic-safe concurrency helpers on top of
// errgroup and plain goroutines, so a bug in one goroutine degrades to a
// logged, reported error instead of crashing the process.
package syncutil

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/Will413028/hawkr/go-pkg/logger"
)

// Group wraps *errgroup.Group so a panic inside Go/TryGo is recovered,
// logged, and turned into an error rather than crashing the process.
// It deliberately does not use errgroup.WithContext: a failed (or
// panicking) goroutine must not cancel its siblings — matches the existing
// best-effort fan-out pattern in gateway/internal/handler.
type Group struct {
	*errgroup.Group
	ctx           context.Context
	panicCallback func(r any)
}

// Option configures a Group.
type Option func(*Group)

// WithPanicCallback registers a callback invoked (in addition to logging)
// whenever a goroutine started via Go/TryGo panics.
func WithPanicCallback(cb func(r any)) Option {
	return func(g *Group) { g.panicCallback = cb }
}

// NewGroup returns a Group. ctx is used only for panic logging (via
// pkg/logger) and must not be nil.
func NewGroup(ctx context.Context, opts ...Option) *Group {
	g := &Group{Group: new(errgroup.Group), ctx: ctx}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Go runs f in a new goroutine, recovering any panic into an error.
func (g *Group) Go(f func() error) {
	g.Group.Go(func() error {
		return recoverPanic(g.ctx, f, g.panicCallback)
	})
}

// TryGo is the non-blocking variant of Go; see errgroup.Group.TryGo.
func (g *Group) TryGo(f func() error) bool {
	return g.Group.TryGo(func() error {
		return recoverPanic(g.ctx, f, g.panicCallback)
	})
}

// SafeGo runs f in a new goroutine, recovering any panic so it cannot crash
// the process. Unlike Go/TryGo, there is no Group to Wait() on — SafeGo is
// for a fire-and-forget call with no result or error to join (e.g. a
// best-effort MQ publish or cache write-back at the end of a request
// handler). Use Group instead when the caller needs to wait for completion
// or collect an error.
//
// ctx is used only for panic logging (via pkg/logger) and must not be nil.
// Since f typically outlives the request that spawned it, pass a ctx
// already detached from the request lifecycle as appropriate (e.g.
// context.WithoutCancel, or a fresh context with its own timeout) — SafeGo
// does not do this for you.
func SafeGo(ctx context.Context, f func()) {
	go func() {
		_ = recoverPanic(ctx, func() error {
			f()
			return nil
		}, nil)
	}()
}

// recoverPanic runs f, converting any panic into an error instead of letting
// it propagate. The panic is logged via pkg/logger and, if cb is non-nil,
// also handed to cb.
func recoverPanic(ctx context.Context, f func() error, cb func(r any)) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(ctx, "syncutil: goroutine panic recovered", zap.Any("recover", r), zap.Stack("stack"))
			if cb != nil {
				cb(r)
			}
			err = fmt.Errorf("syncutil: recovered panic: %v", r)
		}
	}()
	return f()
}
