package runnable

import (
	"context"
	"time"
)

type C interface {
	Add(runner Runnable)
	AddService(service Runnable)
}

type component struct {
	processes *group
	services  *group
}

// Component manages two groups of Runnable, processes and services.
// Services are started before processes, and stopped after them.
// The intended purposes is to manage the simplest form of runnable dependencies.
func Component(processes ...Runnable) *component {
	return &component{
		processes: &group{timeout: 30 * time.Second, runnables: processes},
		services:  &group{timeout: 30 * time.Second},
	}
}

// WithShutdownTimeout changes the default timeout (30s).
func (c *component) WithShutdownTimeout(process, service time.Duration) *component {
	c.processes.timeout = process
	c.services.timeout = service
	return c
}

// Add registers runnables as process. Processes will be shutdown before services.
func (c *component) Add(runner ...Runnable) {
	c.processes.add(runner...)
}

// Add registers runnables as services. Services will be shutdown after processes.
func (c *component) AddService(service ...Runnable) {
	c.services.add(service...)
}

func (c *component) Run(ctx context.Context) error {
	ctxValues := ContextValues(ctx)

	// Starting

	Log(c, "starting services")
	c.services.Start(ctxValues)

	Log(c, "starting processes")
	c.processes.Start(ctxValues)

	// Waiting for shutdown

	select {
	case <-c.processes.WaitForShutdown():
		Log(c, "a process stopped")
	case <-c.services.WaitForShutdown():
		Log(c, "a service stopped")
	case <-ctx.Done():
		Log(c, "context cancelled")
	}

	// shutdown

	Log(c, "shutting down processes")
	c.processes.Stop()

	Log(c, "shutting down services")
	c.services.Stop()

	Log(c, "shutdown complete")
	return ctx.Err()
}
