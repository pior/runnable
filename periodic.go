package runnable

import (
	"context"
	"time"
)

// PeriodicOptions configures the behavior of a Periodic runnable.
type PeriodicOptions struct {
	Period time.Duration
}

// Periodic returns a runnable that will periodically run the runnable passed in argument.
func Periodic(opts PeriodicOptions, runnable Runnable) Runnable {
	return &periodic{opts, runnable}
}

type periodic struct {
	opts     PeriodicOptions
	runnable Runnable
}

func (r *periodic) Run(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()

		err := r.runnable.Run(ctx)
		if err != nil {
			return err
		}

		time.Sleep(time.Until(start.Add(r.opts.Period)))
	}
}

func (r *periodic) name() string {
	return nameOfRunnable(r.runnable)
}
