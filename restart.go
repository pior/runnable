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
// enough run with [ErrorResetAfter].
//
// It loops indefinitely unless limited by [RestartLimit] or [ErrorLimit]. When the
// restart limit is reached, Run returns nil. When the error limit is reached, Run
// returns the last error. Errors that are restarted are logged. Context cancellation stops the loop and returns
// [context.Canceled].
//
//	runnable.Restart(worker, runnable.ErrorLimit(5), runnable.ErrorResetAfter(time.Minute))
func Restart(runnable Runnable, opts ...RestartOption) Runnable {
	r := &restart{
		name:     "restart/" + runnableName(runnable),
		runnable: Recover(runnable),
		errors:   newErrorPolicy(),
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
	name     string
	runnable Runnable
	limit    int
	delay    time.Duration
	errors   errorPolicy
}

var _ Runnable = (*restart)(nil)

func (r *restart) runnableName() string { return r.name }

// RestartLimit sets the maximum number of restarts after successful (nil) exits.
// When reached, [Restart] returns nil. Zero means unlimited (the default).
func RestartLimit(n int) RestartOption {
	return restartOptionFunc(func(r *restart) { r.limit = n })
}

// RestartDelay sets the time to wait before restarting after a successful exit.
// Defaults to zero (immediate restart).
func RestartDelay(d time.Duration) RestartOption {
	return restartOptionFunc(func(r *restart) { r.delay = d })
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
			var limitReached bool
			errorCount, limitReached = r.errors.countError(errorCount, time.Since(startTime))
			if limitReached {
				logger.Info(name+": not restarting", "reason", "error limit", "limit", r.errors.limit)
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
			delay = r.errors.backoff(errorCount)
			logger.Info(name+": failed, restarting", append(errorAttrs(err), "errors", errorCount, "delay", delay)...)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
