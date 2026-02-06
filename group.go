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

	publicStoppedCh chan *StartedRunnable
	stoppedCh       chan *StartedRunnable
}

func (g *group) Add(rs ...Runnable) {
	g.runnables = append(g.runnables, rs...)
}

func (g *group) StoppedRunnables() chan *StartedRunnable {
	return g.publicStoppedCh
}

func (g *group) Start(ctx context.Context) {
	ctx, g.contextCancel = context.WithCancel(ctx)

	g.started = make([]*StartedRunnable, 0, len(g.runnables))
	g.publicStoppedCh = make(chan *StartedRunnable, len(g.runnables))
	g.stoppedCh = make(chan *StartedRunnable, len(g.runnables))

	for _, r := range g.runnables {
		g.started = append(g.started, StartRunnable(ctx, r, g.stoppedCh))
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
			g.publicStoppedCh <- stopped
			running--
			if stopped.err != nil {
				groupErrors = append(groupErrors, stopped.err)
			}
		case <-logTicker.C:
			for _, sr := range g.started {
				stopped, _ := sr.State()
				if !stopped {
					Log(g, "still running: %s", findName(sr))
				}
			}
		case <-stopTimeout:
			Log(g, "waited %s, %d still running", g.timeout, running)
			return
		}
	}

	return
}
