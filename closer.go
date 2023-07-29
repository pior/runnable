package runnable

import (
	"context"
)

// Closer returns a runnable intended to call a Close method on shutdown.
func Closer(c interface{ Close() }) Runnable {
	return &closer{c, func(ctx context.Context) error {
		c.Close()
		return nil
	}}
}

// Closer returns a runnable intended to call a Close method on shutdown.
func CloserErr(c interface{ Close() error }) Runnable {
	return &closer{c, func(ctx context.Context) error {
		return c.Close()
	}}
}

// Closer returns a runnable intended to call a Close method on shutdown.
func CloserCtx(c interface{ Close(context.Context) }) Runnable {
	return &closer{c, func(ctx context.Context) error {
		c.Close(ctx)
		return nil
	}}
}

// Closer returns a runnable intended to call a Close method on shutdown.
func CloserCtxErr(c interface{ Close(context.Context) error }) Runnable {
	return &closer{c, func(ctx context.Context) error {
		return c.Close(ctx)
	}}
}

type closer struct {
	c       any
	closeFn func(context.Context) error
}

func (c *closer) Run(ctx context.Context) error {
	<-ctx.Done()
	err := c.closeFn(ctx)
	if err != nil {
		return &RunnableError{"closer: Close() returned an error", err}
	}
	return nil
}

func (c *closer) name() string {
	return composeName("closer", c.c)
}
