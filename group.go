package runnable

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Group returns a runnable that concurrently runs all runnables passed in argument.
func Group(runnables ...Runnable) *GroupRunner {
	g := &GroupRunner{ShutdownTimeout: time.Second * 5}

	for _, runnable := range runnables {
		g.containers = append(g.containers, &container{
			runnable: Recover(runnable),
		})
	}

	return g
}

type GroupRunner struct {
	containers      []*container
	ShutdownTimeout time.Duration
}

func (g *GroupRunner) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	totalCount := len(g.containers)
	dying := make(chan struct{}, totalCount)
	completedChan := make(chan *container, totalCount)

	// start the runnables in Go routines.
	for _, c := range g.containers {
		c.launch(ctx, completedChan, dying)
		log.Infof("group: %s started\n", c.name())
	}

	// block until group is cancelled, or a runnable dies.
	select {
	case <-ctx.Done():
	case <-dying:
		cancelFunc()
	}

	deadline := time.After(g.ShutdownTimeout)
	completedCount := 0
	shutdown := false

	log.Infof("group: started graceful shutdown (%s)", g.ShutdownTimeout)

	for !shutdown {
		select {
		case c := <-completedChan:
			if c.err == nil {
				log.Infof("group: %s stopped", c.name())
			} else {
				log.Infof("group: %s stopped with %+v", c.name(), c.err)
			}
			completedCount++
		case <-deadline:
			shutdown = true
		}

		if completedCount == totalCount {
			log.Infof("group: shutdown complete")
			break
		}
	}

	errs := []string{}
	for _, c := range g.containers {
		if !c.started {
			continue
		}
		if !c.stopped {
			log.Infof("group: %s is still running", c.name())
			errs = append(errs, fmt.Sprintf("%s is still running", c.name()))
		}
		if c.err != nil {
			errs = append(errs, fmt.Sprintf("%s crashed with %+v", c.name(), c.err))
		}

	}

	if len(errs) != 0 {
		return fmt.Errorf("group: %s", strings.Join(errs, ", "))
	}

	return nil
}

type container struct {
	runnable Runnable
	err      error
	started  bool
	stopped  bool
}

func (c *container) name() string {
	return nameOfRunnable(c.runnable)
}

func (c *container) launch(ctx context.Context, completed chan *container, dying chan struct{}) {
	go func() {
		c.started = true
		c.err = c.runnable.Run(ctx)
		completed <- c
		dying <- struct{}{}
		c.stopped = true
	}()
}
