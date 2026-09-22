package runnable

import (
	"context"
	"time"
)

// Restart returns a runnable that keeps running the given runnable, restarting it
// after both successful exits and errors. Panics are recovered and treated as errors.
//
// On successful exit, the runnable is restarted after [Delay]. On error, it is
// restarted after the backoff of [ErrorBackoff]. The error count tracks
// consecutive errors and resets to zero after any successful run, or after a long
// enough run with [ErrorResetAfter].
//
// It loops indefinitely unless limited by [Limit] or [ErrorLimit]. When the
// restart limit is reached, Run returns nil. When the error limit is reached, Run
// returns the last error. Context cancellation stops the loop and returns
// [context.Canceled].
//
//	runnable.Restart(worker, runnable.ErrorLimit(5), runnable.ErrorResetAfter(time.Minute))
func Restart(runnable Runnable, opts ...RestartOption) Runnable {
	r := &restart{
		name:           "restart/" + runnableName(runnable),
		runnable:       Recover(runnable),
		errorBackoffFn: defaultErrorBackoff,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RestartOption configures [Restart].
type RestartOption func(*restart)

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

// Limit sets the maximum number of restarts after successful (nil) exits.
// When reached, [Restart] returns nil. Zero means unlimited (the default).
func Limit(n int) RestartOption {
	return func(r *restart) { r.limit = n }
}

// ErrorLimit sets the maximum number of consecutive restarts after errors.
// When reached, [Restart] returns the last error. Zero means unlimited (the default).
func ErrorLimit(n int) RestartOption {
	return func(r *restart) { r.errorLimit = n }
}

// Delay sets the time to wait before restarting after a successful exit.
// Defaults to zero (immediate restart).
func Delay(d time.Duration) RestartOption {
	return func(r *restart) { r.delay = d }
}

// ErrorBackoff sets the function that determines the delay before restarting
// after an error. It receives the current consecutive error count (starting at 1).
// The default backs off: immediate for the first 3 errors, 10s up to 10, then 1m.
func ErrorBackoff(fn func(errors int) time.Duration) RestartOption {
	return func(r *restart) { r.errorBackoffFn = fn }
}

// ErrorResetAfter resets the consecutive error count when a single run lasted
// at least the given duration before failing. This prevents long-running services
// that occasionally fail from accumulating stale error counts into the backoff.
// Zero means never reset based on duration (the default). Successful runs always
// reset the error count regardless of this setting.
func ErrorResetAfter(d time.Duration) RestartOption {
	return func(r *restart) { r.errorResetAfter = d }
}

func (r *restart) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)
	restartCount := 0
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

			if r.limit > 0 && restartCount >= r.limit {
				logger.Info(name+": not restarting", "reason", "restart limit", "limit", r.limit)
				return nil
			}
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
