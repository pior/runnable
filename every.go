package runnable

import (
	"context"
	"time"
)

// Every returns a runnable that will periodically run the runnable passed in argument.
func Every(runnable Runnable, period time.Duration) Runnable {
	return &every{
		name:     "every-" + period.String() + "/" + runnableName(runnable),
		runnable: runnable,
		period:   period,
	}
}

type every struct {
	name     string
	runnable Runnable
	period   time.Duration
}

func (e *every) runnableName() string { return e.name }

func (e *every) Run(ctx context.Context) (err error) {
	ticker := time.NewTicker(e.period)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return ctx.Err()
		case <-ticker.C:
			err := e.runnable.Run(ctx)
			if err != nil {
				return err
			}
		}
	}
}
