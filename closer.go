package runnable

import (
	"context"
)

// Closer returns a runnable that calls Close on context cancellation.
// There is no timeout on the call to Close.
func Closer(c interface{ Close() }) Runnable {
	return &closer{"closer/" + runnableName(c), func(_ context.Context) error {
		c.Close()
		return nil
	}}
}

// CloserErr returns a runnable that calls Close on context cancellation.
// There is no timeout on the call to Close.
func CloserErr(c interface{ Close() error }) Runnable {
	return &closer{"closer/" + runnableName(c), func(_ context.Context) error {
		return c.Close()
	}}
}

// CloserCtx returns a runnable that calls Close on context cancellation.
// The context passed to Close is not cancelled, so Close can perform graceful cleanup.
// There is no timeout on the call to Close.
func CloserCtx(c interface{ Close(context.Context) }) Runnable {
	return &closer{"closer/" + runnableName(c), func(ctx context.Context) error {
		c.Close(ctx)
		return nil
	}}
}

// CloserCtxErr returns a runnable that calls Close on context cancellation.
// The context passed to Close is not cancelled, so Close can perform graceful cleanup.
// There is no timeout on the call to Close.
func CloserCtxErr(c interface{ Close(context.Context) error }) Runnable {
	return &closer{"closer/" + runnableName(c), func(ctx context.Context) error {
		return c.Close(ctx)
	}}
}

type closer struct {
	name    string
	closeFn func(context.Context) error
}

func (c *closer) runnableName() string { return c.name }

func (c *closer) Run(ctx context.Context) error {
	<-ctx.Done()
	err := c.closeFn(context.WithoutCancel(ctx))
	if err != nil {
		return &RunnableError{"closer: Close() returned an error", err}
	}
	return nil
}
