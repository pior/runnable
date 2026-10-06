package runnable

import (
	"context"
	"time"
)

// Retry returns a runnable that runs the given runnable again after each error,
// until it succeeds. Panics are recovered and treated as errors.
//
// On error, the runnable runs again after the backoff of [ErrorBackoff], and
// Retry logs the error. It retries indefinitely unless limited by [ErrorLimit]:
// when the limit is reached, Run returns the last error. [ErrorResetAfter]
// resets the error count after a long enough run. [OnError] is called with each
// error. Context cancellation stops the loop and returns the context error.
//
// Unlike [Restart], a successful run is not restarted: Run returns nil.
//
//	runnable.Retry(migrate, runnable.ErrorLimit(5))
func Retry(runnable Runnable, opts ...RetryOption) Runnable {
	r := &retry{
		name:     "retry/" + runnableName(runnable),
		runnable: Recover(runnable),
		errors:   newErrorPolicy(),
	}
	for _, opt := range opts {
		opt.applyRetry(r)
	}
	return r
}

// RetryOption configures [Retry].
type RetryOption interface{ applyRetry(*retry) }

type retryOptionFunc func(*retry)

func (f retryOptionFunc) applyRetry(r *retry) { f(r) }

type retry struct {
	name     string
	runnable Runnable
	errors   errorPolicy
	onError  func(error)
}

var _ Runnable = (*retry)(nil)

func (r *retry) runnableName() string { return r.name }

// OnError sets a function called with each error of the runnable, including the
// last one when [ErrorLimit] is reached. A recovered panic is a [*PanicError].
// It is not called when the run stops on context cancellation.
func OnError(fn func(err error)) RetryOption {
	return retryOptionFunc(func(r *retry) { r.onError = fn })
}

func (r *retry) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)
	errorCount := 0

	for {
		startTime := time.Now()
		err := r.runnable.Run(ctx)

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		if r.onError != nil {
			r.onError(err)
		}

		var limitReached bool
		errorCount, limitReached = r.errors.countError(errorCount, time.Since(startTime))
		if limitReached {
			logger.Info(name+": not retrying", "reason", "error limit", "limit", r.errors.limit)
			return err
		}

		delay := r.errors.backoff(errorCount)
		logger.Info(name+": failed, retrying", append(errorAttrs(err), "errors", errorCount, "delay", delay)...)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
