package runnable

import (
	"context"
	"time"
)

// Restart returns a runnable that keeps running the given runnable, restarting it
// after both successful exits and errors. Panics are recovered and treated as errors.
//
// On successful exit, the runnable is restarted after [restart.Delay] (default: immediate).
// On error, the runnable is restarted after a backoff period determined by
// [restart.ErrorBackoff] (default: immediate for ≤3 errors, 10s for ≤10, then 1m).
//
// The error count tracks consecutive errors and resets to zero after any successful
// run. Use [restart.ErrorResetAfter] to also reset after a run that lasted long
// enough before failing.
//
// Restart loops indefinitely unless limited by [restart.Limit] or [restart.ErrorLimit].
// When the restart limit is reached, Restart returns nil. When the error limit is
// reached, Restart returns the last error.
// Context cancellation stops the loop and returns [context.Canceled].
func Restart(runnable Runnable) *restart {
	return &restart{
		name:           "restart/" + runnableName(runnable),
		runnable:       Recover(runnable),
		errorBackoffFn: defaultErrorBackoff,
	}
}

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
// When reached, returns nil. Zero means unlimited (the default).
func (r *restart) Limit(n int) *restart {
	r.limit = n
	return r
}

// ErrorLimit sets the maximum number of consecutive restarts after errors.
// When reached, returns the last error. Zero means unlimited (the default).
func (r *restart) ErrorLimit(n int) *restart {
	r.errorLimit = n
	return r
}

// Delay sets the time to wait before restarting after a successful exit.
// Defaults to zero (immediate restart).
func (r *restart) Delay(d time.Duration) *restart {
	r.delay = d
	return r
}

// ErrorBackoff sets the function that determines the delay before restarting
// after an error. It receives the current consecutive error count (starting at 1).
// The default backs off: immediate for the first 3 errors, 10s up to 10, then 1m.
func (r *restart) ErrorBackoff(fn func(errors int) time.Duration) *restart {
	r.errorBackoffFn = fn
	return r
}

// ErrorResetAfter resets the consecutive error count when a single run lasted
// at least the given duration before failing. This prevents long-running services
// that occasionally fail from accumulating stale error counts into the backoff.
// Zero means never reset based on duration (the default). Successful runs always
// reset the error count regardless of this setting.
func (r *restart) ErrorResetAfter(d time.Duration) *restart {
	r.errorResetAfter = d
	return r
}

func (r *restart) Run(ctx context.Context) error {
	restartCount := 0
	errorCount := 0

	for {
		logger.Info("starting", "runnable", r.name, "restart", restartCount, "errors", errorCount)

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
				logger.Info("not restarting", "runnable", r.name, "reason", "error limit", "limit", r.errorLimit)
				return err
			}
		} else {
			errorCount = 0

			if r.limit > 0 && restartCount >= r.limit {
				logger.Info("not restarting", "runnable", r.name, "reason", "restart limit", "limit", r.limit)
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
