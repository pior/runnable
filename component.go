package runnable

import (
	"context"
	"time"
)

var DefaultComponentShutdownTimeout = 30 * time.Second

type C interface {
	Add(runner Runnable)
	AddService(service Runnable)
}

type component struct {
	processes *group
	services  *group
}

// Component manages two groups of Runnable, processes and services.
// Services are stopped after processes.
// The intended purposes is to manage the simplest form of runnable dependencies.
func Component(processes ...Runnable) *component {
	return &component{
		processes: &group{timeout: DefaultComponentShutdownTimeout, runnables: processes},
		services:  &group{timeout: DefaultComponentShutdownTimeout},
	}
}

// WithProcessShutdownTimeout changes the shutdown timeout of the process runnables.
// See also DefaultComponentShutdownTimeout.
func (c *component) WithProcessShutdownTimeout(timeout time.Duration) *component {
	c.processes.timeout = timeout
	return c
}

// WithServiceShutdownTimeout changes the shutdown timeout of the service runnables.
// See also DefaultComponentShutdownTimeout.
func (c *component) WithServiceShutdownTimeout(timeout time.Duration) *component {
	c.services.timeout = timeout
	return c
}

// AddProcess registers runnables as process. Processes will be shutdown before services.
func (c *component) AddProcess(runner ...Runnable) *component {
	c.processes.Add(runner...)
	return c
}

// Add registers runnables as services. Services will be shutdown after processes.
func (c *component) AddService(service ...Runnable) *component {
	c.services.Add(service...)
	return c
}

func (c *component) Run(ctx context.Context) error {
	ctxNoCancel := context.WithoutCancel(ctx)

	// Starting

	Log(c, "starting services")
	c.services.Start(ctxNoCancel)

	Log(c, "starting processes")
	c.processes.Start(ctxNoCancel)

	// Waiting for shutdown

	select {
	case r := <-c.processes.StoppedRunnables():
		Log(c, "process stopped: %s", findName(r))
	case r := <-c.services.StoppedRunnables():
		Log(c, "service stopped: %s", findName(r))
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
