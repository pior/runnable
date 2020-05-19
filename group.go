package runnable

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Group returns a runnable that concurrently runs all runnables passed in argument.
// DEPRECATED: use the Manager instead.
func Group(runnables ...Runnable) *GroupRunner {
	g := &GroupRunner{ShutdownTimeout: time.Second * 5}

	for _, runnable := range runnables {
		g.containers = append(g.containers, &container{
			runnable: runnable,
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

	// start the runnables in Go routines (if implemented).
	for _, container := range g.containers {
		err := container.init(ctx)
		if err != nil {
			return err
		}
	}

	// run the runnables in Go routines.
	for _, c := range g.containers {
		c.launch(ctx, completedChan, dying)
		log.Infof("group: %s running\n", c.name())
	}

	// block until group is cancelled, or a runnable dies.
	select {
	case <-ctx.Done():
	case <-dying:
		cancelFunc()
	}

	// graceful shutdown
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
			break
		}
	}

	// start the runnables in Go routines (if implemented).
	for _, container := range reverseContainer(g.containers) {
		container.cleanup(ctx)
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
		if c.err != nil && c.err != context.Canceled {
			errs = append(errs, fmt.Sprintf("%s crashed with %+v", c.name(), c.err))
		}
	}

	log.Infof("group: shutdown complete")

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
	return findName(c.runnable)
}

func (c *container) init(ctx context.Context) error {
	if r, ok := c.runnable.(RunnableInit); ok {
		log.Infof("group: init %s", c.name())
		if err := r.Init(ctx); err != nil {
			return fmt.Errorf("init %s: %w", c.name(), err)
		}
	}
	return nil
}

func (c *container) launch(ctx context.Context, completed chan *container, dying chan struct{}) {
	go func() {
		c.started = true
		c.err = Recover(c.runnable).Run(ctx)
		completed <- c
		dying <- struct{}{}
		c.stopped = true
	}()
}

func (c *container) cleanup(ctx context.Context) {
	if r, ok := c.runnable.(RunnableCleanup); ok {
		log.Infof("group: cleanup %s", c.name())
		err := r.Cleanup(ctx)
		if err != nil {
			log.Warnf("group: failed to cleanup %s", c.name())
		}
	}
}

func reverseContainer(containers []*container) []*container {
	reversed := append([]*container{}, containers...)

	for i, j := 0, len(containers)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return reversed
}
