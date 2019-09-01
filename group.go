package runnable

import (
	"context"
	"log"
	"time"
)

// Group returns a runnable that concurrently runs all runnables passed in argument.
func Group(runnables ...Runnable) Runnable {
	return &group{runnables, time.Second * 5}
}

type group struct {
	runnables       []Runnable
	shutdownTimeout time.Duration
}

func (g *group) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	totalCount := len(g.runnables)

	dying := make(chan struct{}, totalCount)
	completedChan := make(chan *container, totalCount)

	for _, runnable := range g.runnables {
		c := &container{runnable: Recover(runnable)}
		log.Printf("runnable %s started\n", c.name())
		c.launch(ctx, completedChan, dying)
	}

	select {
	case <-ctx.Done():
	case <-dying:
		cancelFunc()
	}

	deadline := time.After(g.shutdownTimeout)
	completedCount := 0
	var err error

	for {
		select {
		case c := <-completedChan:
			log.Printf("runnable %s stopped with %s\n", c.name(), c.err)
			completedCount++
			if c.err != nil {
				err = c.err
			}
		case <-deadline:
			log.Print("runnables took too long to finish")
			return err
		}

		if completedCount == totalCount {
			log.Print("runnables finished")
			break
		}
	}

	return err
}

type container struct {
	runnable Runnable
	err      error
}

func (c *container) name() string {
	return nameOfRunnable(c.runnable)
}

func (c *container) launch(ctx context.Context, completed chan *container, dying chan struct{}) {
	go func() {
		c.err = c.runnable.Run(ctx)
		completed <- c
		dying <- struct{}{}
	}()
}
