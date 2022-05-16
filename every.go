package runnable

import (
	"context"
	"fmt"
	"time"
)

// Every returns a runnable that will periodically run the runnable passed in argument.
func Every(runnable Runnable, period time.Duration) Runnable {
	return &every{runnable, period}
}

type every struct {
	runnable Runnable
	period   time.Duration
}

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

func (e *every) name() string {
	return composeName(fmt.Sprintf("every[%s]", e.period), e.runnable)
}
