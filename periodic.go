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
func Periodic(opts PeriodicOptions, runnable Runnable) *periodic {
	return &periodic{opts, runnable}
}

type periodic struct {
	opts     PeriodicOptions
	runnable Runnable
}

func (r *periodic) Run(ctx context.Context) (err error) {
	ticker := time.NewTicker(r.opts.Period)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return ctx.Err()
		case <-ticker.C:
			err := r.runnable.Run(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (r *periodic) name() string {
	return composeName("periodic", r.runnable)
}
