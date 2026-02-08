package runnable

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Manager returns a new manager that coordinates the lifecycle of multiple runnables.
//
// Runnables are organized in two tiers: processes (foreground work) and services
// (infrastructure like databases or queues). Shutdown is triggered when the context
// is cancelled or any runnable completes. During shutdown, processes are cancelled
// first, then services, ensuring services remain available while processes drain.
//
// Each runnable is wrapped with [Recover] to catch panics. Errors from runnables are
// collected, except [context.Canceled] which is ignored. A manager is itself a
// [Runnable], so managers can be nested for independent shutdown ordering.
//
// Registering the same runnable twice, or as both a process and a service, panics.
func Manager() *manager {
	return &manager{
		name:            "manager",
		shutdownTimeout: 10 * time.Second,
	}
}

type manager struct {
	name            string
	processes       []Runnable
	services        []Runnable
	shutdownTimeout time.Duration
}

func (m *manager) runnableName() string { return m.name }

// Name sets the manager's name, used as a prefix in log messages.
func (m *manager) Name(name string) *manager {
	m.name = name
	return m
}

// ShutdownTimeout sets the maximum time allowed for each shutdown phase.
// Defaults to 10 seconds.
func (m *manager) ShutdownTimeout(dur time.Duration) *manager {
	m.shutdownTimeout = dur
	return m
}

// ManagerRegistry is the interface for registering runnables with a Manager.
type ManagerRegistry interface {
	// Register registers processes. Processes are the primary runnables of the
	// application. They are cancelled first during shutdown.
	Register(runners ...Runnable) ManagerRegistry
	// RegisterService registers services. Services are infrastructure runnables
	// (databases, queues, etc.) that processes depend on. They are cancelled after
	// all processes have stopped.
	RegisterService(services ...Runnable) ManagerRegistry
}

var _ ManagerRegistry = (*manager)(nil)

// Register registers processes. Processes are the primary runnables of the
// application. They are cancelled first during shutdown.
// Panics if any runnable is already registered.
func (m *manager) Register(runners ...Runnable) ManagerRegistry {
	for _, r := range runners {
		if slices.Contains(m.processes, r) || slices.Contains(m.services, r) {
			panic(fmt.Sprintf("runnable %s already registered", runnableName(r)))
		}
	}
	m.processes = append(m.processes, runners...)
	return m
}

// RegisterService registers services. Services are infrastructure runnables
// (databases, queues, etc.) that processes depend on. They are cancelled after
// all processes have stopped.
// Panics if any runnable is already registered.
func (m *manager) RegisterService(services ...Runnable) ManagerRegistry {
	for _, s := range services {
		if slices.Contains(m.processes, s) || slices.Contains(m.services, s) {
			panic(fmt.Sprintf("runnable %s already registered", runnableName(s)))
		}
	}
	m.services = append(m.services, services...)
	return m
}

type completed struct {
	runnable Runnable
	err      error
}

type runnableSet map[Runnable]bool

func (m *manager) Run(ctx context.Context) error {
	prefix := m.runnableName()

	svcCtx, svcCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer svcCancel()

	procCtx, procCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer procCancel()

	svcDone := make(chan completed, len(m.services))
	procDone := make(chan completed, len(m.processes))

	for _, svc := range m.services {
		go func() {
			svcDone <- completed{svc, Recover(svc).Run(svcCtx)}
		}()
		logger.Info(prefix + "/" + runnableName(svc) + ": started")
	}

	for _, proc := range m.processes {
		go func() {
			procDone <- completed{proc, Recover(proc).Run(procCtx)}
		}()
		logger.Info(prefix + "/" + runnableName(proc) + ": started")
	}

	// Track completed runnables from the initial trigger.
	var errs []string
	activeProcs := runnableSet{}
	for _, p := range m.processes {
		activeProcs[p] = true
	}
	activeSvcs := runnableSet{}
	for _, s := range m.services {
		activeSvcs[s] = true
	}

	// Wait for context cancellation or any runnable to complete.
	select {
	case <-ctx.Done():
		logger.Info(prefix+": starting shutdown", "reason", "context cancelled")
	case c := <-procDone:
		delete(activeProcs, c.runnable)
		m.logCompleted(c)
		m.collectError(&errs, c)
		logger.Info(prefix+": starting shutdown", "reason", runnableName(c.runnable)+" died")
	case c := <-svcDone:
		delete(activeSvcs, c.runnable)
		m.logCompleted(c)
		m.collectError(&errs, c)
		logger.Info(prefix+": starting shutdown", "reason", runnableName(c.runnable)+" died")
	}

	// Phase 1: stop processes
	procCancel()

	deadline := time.After(m.shutdownTimeout)

	for len(activeProcs) > 0 {
		select {
		case c := <-procDone:
			delete(activeProcs, c.runnable)
			m.logCompleted(c)
			m.collectError(&errs, c)
		case <-deadline:
			for p := range activeProcs {
				logger.Info(prefix + "/" + runnableName(p) + ": still running")
				errs = append(errs, fmt.Sprintf("%s is still running", runnableName(p)))
			}
			activeProcs = nil
		}
	}

	// Phase 2: stop services
	svcCancel()

	deadline = time.After(m.shutdownTimeout)

	for len(activeSvcs) > 0 {
		select {
		case c := <-svcDone:
			delete(activeSvcs, c.runnable)
			m.logCompleted(c)
			m.collectError(&errs, c)
		case <-deadline:
			for s := range activeSvcs {
				logger.Info(prefix + "/" + runnableName(s) + ": still running")
				errs = append(errs, fmt.Sprintf("%s is still running", runnableName(s)))
			}
			activeSvcs = nil
		}
	}

	logger.Info(prefix + ": shutdown complete")

	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", prefix, strings.Join(errs, ", "))
	}
	return nil
}

func (m *manager) logCompleted(c completed) {
	name := m.runnableName() + "/" + runnableName(c.runnable)
	if c.err == nil || errors.Is(c.err, context.Canceled) {
		logger.Info(name + ": stopped")
	} else {
		logger.Info(name+": stopped with error", "error", c.err)
	}
}

func (m *manager) collectError(errs *[]string, c completed) {
	if c.err != nil && !errors.Is(c.err, context.Canceled) {
		*errs = append(*errs, fmt.Sprintf("%s crashed with %+v", runnableName(c.runnable), c.err))
	}
}
