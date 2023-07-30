package runnable

import (
	"context"
	"time"
)

type restartConfig struct {
	restartLimit        int
	crashLimit          int
	restartDelay        time.Duration
	crashBackoffDelayFn func(int) time.Duration
}

type RestartOption func(*restartConfig)

// RestartLimit sets a limit on the number of restart after successful execution.
func RestartLimit(times int) RestartOption {
	return func(cfg *restartConfig) {
		cfg.restartLimit = times
	}
}

// RestartCrashLimit sets a limit on the number restart after a crash.
func RestartCrashLimit(times int) RestartOption {
	return func(cfg *restartConfig) {
		cfg.crashLimit = times
	}
}

// RestartDelay sets the time waited before restarting the runnable after a successful execution.
func RestartDelay(delay time.Duration) RestartOption {
	return func(cfg *restartConfig) {
		cfg.restartDelay = delay
	}
}

// RestartCrashDelayFn sets the function that determine the backoff delay after a crash.
func RestartCrashDelayFn(fn func(int) time.Duration) RestartOption {
	return func(cfg *restartConfig) {
		cfg.crashBackoffDelayFn = fn
	}
}

// Restart returns a runnable that runs a runnable and restarts it when it fails, with some conditions.
func Restart(runnable Runnable, opts ...RestartOption) Runnable {
	cfg := restartConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.crashBackoffDelayFn == nil {
		cfg.crashBackoffDelayFn = crashBackoffDelay
	}
	return &restart{baseWrapper{"restart", runnable}, runnable, cfg}
}

type restart struct {
	baseWrapper
	runnable Runnable
	cfg      restartConfig
}

func (r *restart) Run(ctx context.Context) error {
	restartCount := 0
	crashCount := 0

	for {
		Log(r, "starting (restart=%d crash=%d)", restartCount, crashCount)
		err := r.runnable.Run(ctx)
		isCrash := err != nil

		if isCrash {
			crashCount++
		}

		if r.cfg.restartLimit > 0 && restartCount >= r.cfg.restartLimit {
			Log(r, "not restarting (hit the restart limit: %d)", r.cfg.restartLimit)
			return err
		}

		if r.cfg.crashLimit > 0 && crashCount >= r.cfg.crashLimit {
			Log(r, "not restarting (hit the crash limit: %d)", r.cfg.crashLimit)
			return err
		}

		restartCount++

		delay := r.cfg.restartDelay
		if isCrash {
			delay = r.cfg.crashBackoffDelayFn(crashCount)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func crashBackoffDelay(crashCount int) time.Duration {
	if crashCount <= 3 {
		return 0
	} else if crashCount <= 10 {
		return time.Second * 10
	} else {
		return time.Minute
	}
}
