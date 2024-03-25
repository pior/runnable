package runnable

import "context"

// Noop returns a runnable that does nothing, and return when the context is cancelled.
func Noop() Runnable {
	return Func(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
}
