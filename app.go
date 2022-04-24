package runnable

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type App struct {
	components []*component
}

func NewApp() *App {
	return &App{}
}

func (a *App) Add(runnable Runnable, opts ...AppOption) {
	for _, c := range a.components {
		if c.runnable == runnable {
			panic(fmt.Sprintf("runnable.App: runnable %T already added", runnable))
		}
	}

	c := &component{runnable: runnable}
	applyAppOptions(c, opts)

	a.components = append(a.components, c)
}

func (a *App) Run(ctx context.Context) error {
	if len(a.components) == 0 {
		return errors.New("runnable.App: no runnable defined")
	}

	died := make(chan *component, len(a.components))
	stages := prepareStages(ContextValues(ctx), a.components)

	startComponent := func(st *stage, c *component) {
		log.Infof("started[%d]: %s", st.order, findName(c.runnable))

		c.err = c.runnable.Run(st.ctx)
		if c.err == nil || errors.Is(c.err, context.Canceled) {
			log.Infof("stopped[%d]: %s", st.order, findName(c.runnable))
		} else {
			log.Infof("stopped[%d]: %s (%s)", st.order, findName(c.runnable), c.err)
		}

		c.stopped.setTrue()
		died <- c
	}

	// - startup by stage
	for _, st := range stages.fromLowToHighOrders() {
		for _, c := range st.components {
			go startComponent(st, c)
		}
		time.Sleep(100 * time.Millisecond)
	}

	log.Infof("startup complete")

	// - wait until shutdown
	select {
	case <-ctx.Done():
		log.Infof("shutdown started")
	case c := <-died:
		log.Infof("shutdown started (%s stopped)", findName(c.runnable))
	}

	// - shutdown
	for _, stage := range stages.fromHighToLowOrders() {
		stage.cancel()

		for {
			running := stage.componentsRunning()
			if len(running) == 0 {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Infof("shutdown complete")
	return nil
}

type component struct {
	order int

	runnable Runnable

	stopped atomicBool
	err     error
}
