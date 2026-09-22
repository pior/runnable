package runnable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
	processes       []entry
	services        []entry
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
// Panics if any runnable is already registered. Duplicate detection only applies
// to comparable runnables, in practice pointers.
func (m *manager) Register(runners ...Runnable) ManagerRegistry {
	for _, r := range runners {
		m.processes = append(m.processes, m.newEntry(r))
	}
	return m
}

// RegisterService registers services. Services are infrastructure runnables
// (databases, queues, etc.) that processes depend on. They are cancelled after
// all processes have stopped.
// Panics if any runnable is already registered. Duplicate detection only applies
// to comparable runnables, in practice pointers.
func (m *manager) RegisterService(services ...Runnable) ManagerRegistry {
	for _, s := range services {
		m.services = append(m.services, m.newEntry(s))
	}
	return m
}

// entry is a registered runnable with its name computed once at registration.
type entry struct {
	runnable Runnable
	name     string
}

// newEntry builds an entry for r, panicking if r is already registered.
func (m *manager) newEntry(r Runnable) entry {
	if m.isRegistered(r) {
		panic(fmt.Sprintf("runnable %s already registered", runnableName(r)))
	}
	return entry{runnable: r, name: runnableName(r)}
}

// isRegistered reports whether r is already registered. Comparing interface values
// holding a non-comparable type panics, so those are never considered duplicates.
func (m *manager) isRegistered(r Runnable) bool {
	if !reflect.TypeOf(r).Comparable() {
		return false
	}
	for _, e := range slices.Concat(m.processes, m.services) {
		if e.runnable == r {
			return true
		}
	}
	return false
}

// completed reports the result of the entry at index in its slice.
type completed struct {
	index int
	err   error
}

// activeSet tracks which entries of a slice are still running, by index.
type activeSet struct {
	running []bool
	count   int
}

func newActiveSet(n int) *activeSet {
	running := make([]bool, n)
	for i := range running {
		running[i] = true
	}
	return &activeSet{running: running, count: n}
}

func (a *activeSet) done(i int) {
	if a.running[i] {
		a.running[i] = false
		a.count--
	}
}

func (m *manager) Run(ctx context.Context) error {
	prefix := m.runnableName()

	svcCtx, svcCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer svcCancel()

	procCtx, procCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer procCancel()

	svcDone := make(chan completed, len(m.services))
	procDone := make(chan completed, len(m.processes))

	for i, svc := range m.services {
		go func() {
			svcDone <- completed{i, Recover(svc.runnable).Run(svcCtx)}
		}()
		logger.Info(prefix + "/" + svc.name + ": started")
	}

	for i, proc := range m.processes {
		go func() {
			procDone <- completed{i, Recover(proc.runnable).Run(procCtx)}
		}()
		logger.Info(prefix + "/" + proc.name + ": started")
	}

	// Track completed runnables from the initial trigger.
	var errs []string
	activeProcs := newActiveSet(len(m.processes))
	activeSvcs := newActiveSet(len(m.services))

	// Wait for context cancellation or any runnable to complete.
	select {
	case <-ctx.Done():
		logger.Info(prefix+": starting shutdown", "reason", "context cancelled")
	case c := <-procDone:
		e := m.processes[c.index]
		activeProcs.done(c.index)
		m.logCompleted(e, c.err)
		m.collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", e.name+" died")
	case c := <-svcDone:
		e := m.services[c.index]
		activeSvcs.done(c.index)
		m.logCompleted(e, c.err)
		m.collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", e.name+" died")
	}

	// Phase 1: stop processes
	procCancel()
	m.waitPhase(m.processes, activeProcs, procDone, time.After(m.shutdownTimeout), &errs)

	// Phase 2: stop services
	svcCancel()
	m.waitPhase(m.services, activeSvcs, svcDone, time.After(m.shutdownTimeout), &errs)

	logger.Info(prefix + ": shutdown complete")

	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", prefix, strings.Join(errs, ", "))
	}
	return nil
}

// waitPhase waits for all active entries to complete, or for the deadline.
func (m *manager) waitPhase(
	entries []entry,
	active *activeSet,
	done <-chan completed,
	deadline <-chan time.Time,
	errs *[]string,
) {
	for active.count > 0 {
		select {
		case c := <-done:
			e := entries[c.index]
			active.done(c.index)
			m.logCompleted(e, c.err)
			m.collectError(errs, e, c.err)
		case <-deadline:
			for i, running := range active.running {
				if running {
					logger.Info(m.runnableName() + "/" + entries[i].name + ": still running")
					*errs = append(*errs, fmt.Sprintf("%s is still running", entries[i].name))
				}
			}
			return
		}
	}
}

func (m *manager) logCompleted(e entry, err error) {
	name := m.runnableName() + "/" + e.name
	if err == nil || errors.Is(err, context.Canceled) {
		logger.Info(name + ": stopped")
	} else {
		logger.Info(name+": stopped with error", "error", err)
	}
}

func (m *manager) collectError(errs *[]string, e entry, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		*errs = append(*errs, fmt.Sprintf("%s crashed with %+v", e.name, err))
	}
}
