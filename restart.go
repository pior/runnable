package runnable

import (
	"context"
	"time"
)

// RestartOptions configures the behavior of the Restart runnable
type RestartOptions struct {
	RestartLimit int // Number of restart after which the runnable will not be restarted.
	CrashLimit   int // Number of crash after which the runnable will not be restarted.

	BackoffSleep      time.Duration // Time to wait before restarting.
	CrashBackoffSleep time.Duration // Time to wait before restarting after a crash

	// CrashLoopRestartLimit int           // Number of restarts in CrashLoopPeriod that triggers the crash loop state.
	// CrashLoopPeriod       time.Duration // Observation period for the crash loop detection.
	// CrashLoopBackoffSleep time.Duration // Time to wait before restarting in crash loop.
}

// Restart returns a runnable that runs a runnable and restarts it when it fails, with some conditions.
func Restart(opts RestartOptions, runnable Runnable) Runnable {
	return &restart{opts, runnable}
}

type restart struct {
	opts     RestartOptions
	runnable Runnable
}

func (r *restart) Run(ctx context.Context) error {
	restartCount := 0
	crashCount := 0

	for {
		err := r.runnable.Run(ctx)
		if err != nil {
			crashCount++
		}

		if r.opts.RestartLimit > 0 && restartCount >= r.opts.RestartLimit {
			return err
		}

		if r.opts.CrashLimit > 0 && crashCount >= r.opts.CrashLimit {
			return err
		}

		restartCount++

		if err != nil {
			time.Sleep(r.opts.CrashBackoffSleep)
		} else {
			time.Sleep(r.opts.BackoffSleep)
		}
	}
}
