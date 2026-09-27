package runnable

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Timeout returns a runnable that cancels the given runnable when it runs for
// longer than d, and returns an error wrapping [context.DeadlineExceeded].
//
// When the runnable stops on the timeout with nil or [context.DeadlineExceeded],
// Timeout returns "timed out after <d>: context deadline exceeded". Any other
// error is returned as is. A cancellation of the parent context is not a
// timeout: the result of the runnable is returned as is.
//
//	runnable.Schedule(runnable.Timeout(time.Minute, job), runnable.Every(time.Hour))
func Timeout(d time.Duration, r Runnable) Runnable {
	return &timeout{name: "timeout/" + runnableName(r), timeout: d, runnable: r}
}

type timeout struct {
	name     string
	timeout  time.Duration
	runnable Runnable
}

var _ Runnable = (*timeout)(nil)

func (r *timeout) runnableName() string { return r.name }

// errTimedOut is the cause of the context cancelled by [Timeout], to tell it
// apart from a deadline of the parent context.
var errTimedOut = errors.New("timed out")

func (r *timeout) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeoutCause(ctx, r.timeout, errTimedOut)
	defer cancel()

	err := r.runnable.Run(ctx)

	if !errors.Is(context.Cause(ctx), errTimedOut) {
		return err
	}
	logger.Info(resolveName(ctx, r.name)+": timed out", "timeout", r.timeout)

	if err == nil || err == context.DeadlineExceeded { //nolint:errorlint // only the bare ctx.Err() is replaced
		return fmt.Errorf("timed out after %s: %w", r.timeout, context.DeadlineExceeded)
	}
	return err
}
