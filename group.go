package runnable

import (
	"context"
	"time"
)

type group struct {
	runnables []Runnable
	timeout   time.Duration

	contextCancel func()
	started       []*StartedRunnable
	shutdownCh    chan struct{}
	stoppedCh     chan *StartedRunnable
}

func (g *group) add(rs ...Runnable) {
	g.runnables = append(g.runnables, rs...)
}

// func (g *group) Run(ctx context.Context) error {
// 	g.Start(ctx)

// 	select {
// 	case <-ctx.Done():
// 	case <-g.shutdownCh:
// 	}

// 	g.waitCh <- struct{}{}

// 	errs := append([]error{ctx.Err()}, g.Stop()...)
// 	return errors.Join(errs...)
// }

func (g *group) WaitForShutdown() chan struct{} {
	return g.shutdownCh
}

func (g *group) Start(ctx context.Context) {
	ctx, g.contextCancel = context.WithCancel(ctx)

	g.started = make([]*StartedRunnable, 0, len(g.runnables))
	g.shutdownCh = make(chan struct{}, len(g.runnables))
	g.stoppedCh = make(chan *StartedRunnable, len(g.runnables))

	for _, r := range g.runnables {
		g.started = append(g.started, StartRunnable(ctx, r, g.shutdownCh, g.stoppedCh))
	}

}

func (g *group) Stop() (groupErrors []error) {
	g.contextCancel() // stop all other runnings

	stopTimeout := time.After(g.timeout)
	logTicker := time.NewTicker(3 * time.Second)
	defer logTicker.Stop()

	running := len(g.started)
	for running > 0 {
		select {
		case stopped := <-g.stoppedCh:
			running--
			if stopped.err != nil {
				groupErrors = append(groupErrors, stopped.err)
			}
		case <-logTicker.C:
			Log(g, "%d still running", running)
		case <-stopTimeout:
			Log(g, "waited %s, %d still running", g.timeout, running)
			return
		}
	}

	return
}
