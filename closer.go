package runnable

import (
	"context"
	"io"
	"time"
)

// CloserOptions configures the behavior of a Closer runnable.
type CloserOptions struct {
	Delay time.Duration
}

// Closer returns a runnable that will close what is passed in argument.
func Closer(object io.Closer, opts *CloserOptions) Runnable {
	if opts == nil {
		opts = &CloserOptions{}
	}
	return &closer{opts, object}
}

type closer struct {
	opts   *CloserOptions
	object io.Closer
}

func (c *closer) Run(ctx context.Context) error {
	<-ctx.Done()
	time.Sleep(c.opts.Delay)
	err := c.object.Close()
	if err != nil {
		return &RunnableError{"closer: Close() returned an error", err}
	}
	return nil
}

func (c *closer) name() string {
	return composeName("closer", c.object)
}
