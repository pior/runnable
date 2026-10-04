package runnable

import (
	"context"
	"time"
)

// Restart returns a runnable that keeps running the given runnable, restarting it
// after both successful exits and errors. Panics are recovered and treated as errors.
//
// On successful exit, the runnable is restarted after [RestartDelay]. On error, it is
// restarted after the backoff of [ErrorBackoff]. The error count tracks
// consecutive errors and resets to zero after any successful run, or after a long
// enough run with [RestartErrorResetAfter].
//
// It loops indefinitely unless limited by [RestartLimit] or [RestartErrorLimit]. When the
// restart limit is reached, Run returns nil. When the error limit is reached, Run
// returns the last error. Context cancellation stops the loop and returns
// [context.Canceled].
//
//	runnable.Restart(worker, runnable.RestartErrorLimit(5), runnable.RestartErrorResetAfter(time.Minute))
func Restart(runnable Runnable, opts ...RestartOption) Runnable {
	r := &restart{
		name:           "restart/" + runnableName(runnable),
		runnable:       Recover(runnable),
		errorBackoffFn: defaultErrorBackoff,
	}
	for _, opt := range opts {
		opt.applyRestart(r)
	}
	return r
}

// RestartOption configures [Restart].
type RestartOption interface{ applyRestart(*restart) }

type restartOptionFunc func(*restart)

func (f restartOptionFunc) applyRestart(r *restart) { f(r) }

type restart struct {
	name            string
	runnable        Runnable
	limit           int
	errorLimit      int
	delay           time.Duration
	errorBackoffFn  func(int) time.Duration
	errorResetAfter time.Duration
}

var _ Runnable = (*restart)(nil)

func (r *restart) runnableName() string { return r.name }

// RestartLimit sets the maximum number of restarts after successful (nil) exits.
// When reached, [Restart] returns nil. Zero means unlimited (the default).
func RestartLimit(n int) RestartOption {
	return restartOptionFunc(func(r *restart) { r.limit = n })
}

// RestartErrorLimit sets the maximum number of consecutive restarts after errors.
// When reached, [Restart] returns the last error. Zero means unlimited (the default).
func RestartErrorLimit(n int) RestartOption {
	return restartOptionFunc(func(r *restart) { r.errorLimit = n })
}

// RestartDelay sets the time to wait before restarting after a successful exit.
// Defaults to zero (immediate restart).
func RestartDelay(d time.Duration) RestartOption {
	return restartOptionFunc(func(r *restart) { r.delay = d })
}

// BackoffOption configures the error backoff of [Restart] and [Retry].
type BackoffOption interface {
	RestartOption
	RetryOption
}

var _ BackoffOption = errorBackoff(nil)

// ErrorBackoff sets the function that determines the delay before running again
// after an error, for [Restart] and [Retry]. It receives the current consecutive
// error count (starting at 1). The default backs off: immediate for the first 3
// errors, 10s up to 10, then 1m.
func ErrorBackoff(fn func(errors int) time.Duration) BackoffOption {
	return errorBackoff(fn)
}

type errorBackoff func(errors int) time.Duration

func (f errorBackoff) applyRestart(r *restart) { r.errorBackoffFn = f }

func (f errorBackoff) applyRetry(r *retry) { r.errorBackoffFn = f }

// RestartErrorResetAfter resets the consecutive error count when a single run lasted
// at least the given duration before failing. This prevents long-running services
// that occasionally fail from accumulating stale error counts into the backoff.
// Zero means never reset based on duration (the default). Successful runs always
// reset the error count regardless of this setting.
func RestartErrorResetAfter(d time.Duration) RestartOption {
	return restartOptionFunc(func(r *restart) { r.errorResetAfter = d })
}

func (r *restart) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)
	restartCount := 0
	successCount := 0
	errorCount := 0

	for {
		logger.Info(name+": starting", "restart", restartCount, "errors", errorCount)

		startTime := time.Now()
		err := r.runnable.Run(ctx)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err != nil {
			if r.errorResetAfter > 0 && time.Since(startTime) >= r.errorResetAfter {
				errorCount = 0
			}
			errorCount++

			if r.errorLimit > 0 && errorCount >= r.errorLimit {
				logger.Info(name+": not restarting", "reason", "error limit", "limit", r.errorLimit)
				return err
			}
		} else {
			errorCount = 0

			if r.limit > 0 && successCount >= r.limit {
				logger.Info(name+": not restarting", "reason", "restart limit", "limit", r.limit)
				return nil
			}
			successCount++
		}

		restartCount++

		delay := r.delay
		if err != nil {
			delay = r.errorBackoffFn(errorCount)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func defaultErrorBackoff(errorCount int) time.Duration {
	switch {
	case errorCount <= 3:
		return 0
	case errorCount <= 10:
		return 10 * time.Second
	default:
		return time.Minute
	}
}
