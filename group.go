package runnable

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type group struct {
	runnables []Runnable
	running   uint32
	errors    chan error
	cancel    func()
	timeout   time.Duration
}

func (g *group) start(ctx context.Context) {
	newCtx, cancel := context.WithCancel(ctx)

	g.cancel = cancel
	g.errors = make(chan error)

	for _, r := range g.runnables {
		atomic.AddUint32(&g.running, 1)
		go g.run(newCtx, r)
	}
}

func (g *group) stop() {
	g.cancel()

	if g.running == 0 {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	after := time.After(g.timeout)

	for {
		select {
		case <-ticker.C:
			Log(g, "%d still running", g.running)
		case <-g.errors:
			if g.running == 0 {
				return
			}
		case <-after:
			Log(g, "waited %s, %d still running", g.timeout, g.running)
			return
		}
	}
}

func (g *group) run(ctx context.Context, runnable Runnable) {
	name := findName(runnable)

	log.Printf("group: %s started", name)
	err := Recover(runnable).Run(ctx)
	if err == nil || errors.Is(err, context.Canceled) {
		log.Printf("group: %s stopped", name)
	} else {
		log.Printf("group: %s stopped with error: %+v", name, err)
	}

	atomic.AddUint32(&g.running, ^uint32(0))
	g.errors <- err
}
