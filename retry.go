package runnable

import (
	"context"
	"time"
)

// Retry returns a runnable that runs the given runnable again after each error,
// until it succeeds. Panics are recovered and treated as errors.
//
// On error, the runnable runs again after the backoff of [ErrorBackoff], and
// Retry logs the error. It retries indefinitely unless limited by [RetryLimit]:
// when the limit is reached, Run returns the last error. Context cancellation
// stops the loop and returns the context error.
//
// Unlike [Restart], a successful run is not restarted: Run returns nil.
//
//	runnable.Retry(migrate, runnable.RetryLimit(5))
func Retry(runnable Runnable, opts ...RetryOption) Runnable {
	r := &retry{
		name:           "retry/" + runnableName(runnable),
		runnable:       Recover(runnable),
		errorBackoffFn: defaultErrorBackoff,
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
	name           string
	runnable       Runnable
	limit          int
	errorBackoffFn func(int) time.Duration
}

var _ Runnable = (*retry)(nil)

func (r *retry) runnableName() string { return r.name }

// RetryLimit sets the maximum number of retries after errors: the runnable runs
// at most n+1 times. When reached, [Retry] returns the last error. Zero means
// unlimited (the default).
func RetryLimit(n int) RetryOption {
	return retryOptionFunc(func(r *retry) { r.limit = n })
}

func (r *retry) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)

	for errorCount := 1; ; errorCount++ {
		err := r.runnable.Run(ctx)

		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		if r.limit > 0 && errorCount > r.limit {
			logger.Info(name+": not retrying", "reason", "retry limit", "limit", r.limit)
			return err
		}

		delay := r.errorBackoffFn(errorCount)
		logger.Info(name+": failed, retrying", "error", err, "errors", errorCount, "delay", delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
