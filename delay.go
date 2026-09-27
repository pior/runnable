package runnable

import (
	"context"
	"time"
)

// Delay returns a runnable that waits for d before running the given runnable,
// for example to stagger the start of jobs. Context cancellation during the wait
// returns the context error without running it.
//
//	runnable.Delay(30*time.Second, warmup)
func Delay(d time.Duration, r Runnable) Runnable {
	return &delay{name: "delay/" + runnableName(r), delay: d, runnable: r}
}

type delay struct {
	name     string
	delay    time.Duration
	runnable Runnable
}

var _ Runnable = (*delay)(nil)

func (r *delay) runnableName() string { return r.name }

func (r *delay) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)

	logger.Info(name+": waiting", "delay", r.delay)
	timer := time.NewTimer(r.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	logger.Info(name + ": starting")
	return r.runnable.Run(ctx)
}
