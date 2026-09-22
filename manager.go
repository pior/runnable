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

// Manager coordinates the lifecycle of multiple runnables. Build it with [NewManager].
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
type Manager struct {
	name            string
	processes       []entry
	services        []entry
	shutdownTimeout time.Duration
}

// NewManager returns a new [Manager].
func NewManager() *Manager {
	return &Manager{
		name:            "manager",
		shutdownTimeout: 10 * time.Second,
	}
}

func (m *Manager) runnableName() string { return m.name }

// Name sets the manager's name, used as a prefix in log messages.
func (m *Manager) Name(name string) *Manager {
	m.name = name
	return m
}

// ShutdownTimeout sets the maximum time allowed for each shutdown phase.
// Defaults to 10 seconds.
func (m *Manager) ShutdownTimeout(dur time.Duration) *Manager {
	m.shutdownTimeout = dur
	return m
}

// ManagerRegistry is the interface for registering runnables with a [Manager].
// It lets helpers register runnables without being able to run the manager.
type ManagerRegistry interface {
	// RegisterProcess registers processes. Processes are the primary runnables of the
	// application. They are cancelled first during shutdown.
	RegisterProcess(processes ...Runnable)
	// RegisterService registers services. Services are infrastructure runnables
	// (databases, queues, etc.) that processes depend on. They are cancelled after
	// all processes have stopped.
	RegisterService(services ...Runnable)
}

var (
	_ Runnable        = (*Manager)(nil)
	_ ManagerRegistry = (*Manager)(nil)
)

// RegisterProcess registers processes. Processes are the primary runnables of the
// application. They are cancelled first during shutdown.
// Panics if any runnable is already registered. Duplicate detection only applies
// to comparable runnables, in practice pointers.
func (m *Manager) RegisterProcess(processes ...Runnable) {
	for _, p := range processes {
		m.processes = append(m.processes, m.newEntry(p))
	}
}

// RegisterService registers services. Services are infrastructure runnables
// (databases, queues, etc.) that processes depend on. They are cancelled after
// all processes have stopped.
// Panics if any runnable is already registered. Duplicate detection only applies
// to comparable runnables, in practice pointers.
func (m *Manager) RegisterService(services ...Runnable) {
	for _, s := range services {
		m.services = append(m.services, m.newEntry(s))
	}
}

// entry is a registered runnable with its name computed once at registration.
type entry struct {
	runnable Runnable
	name     string
	// stopped is run state: reset when Run starts, set when the runnable returns.
	// Concurrent Run calls on the same manager are not supported.
	stopped bool
}

// newEntry builds an entry for r, panicking if r is already registered.
func (m *Manager) newEntry(r Runnable) entry {
	if m.isRegistered(r) {
		panic(fmt.Sprintf("runnable %s already registered", runnableName(r)))
	}
	return entry{runnable: r, name: runnableName(r)}
}

// isRegistered reports whether r is already registered. Comparing interface values
// holding a non-comparable type panics, so those are never considered duplicates.
func (m *Manager) isRegistered(r Runnable) bool {
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

func (m *Manager) Run(ctx context.Context) error {
	prefix := m.runnableName()

	svcCtx, svcCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer svcCancel()

	procCtx, procCancel := context.WithCancel(context.WithoutCancel(ctx))
	defer procCancel()

	svcDone := make(chan completed, len(m.services))
	procDone := make(chan completed, len(m.processes))

	for i := range m.services {
		m.services[i].stopped = false
	}
	for i := range m.processes {
		m.processes[i].stopped = false
	}

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

	var errs []string

	// Wait for context cancellation or any runnable to complete.
	select {
	case <-ctx.Done():
		logger.Info(prefix+": starting shutdown", "reason", "context cancelled")
	case c := <-procDone:
		e := m.markStopped(m.processes, c)
		m.collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", e.name+" died")
	case c := <-svcDone:
		e := m.markStopped(m.services, c)
		m.collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", e.name+" died")
	}

	// Phase 1: stop processes
	procCancel()
	m.waitPhase(m.processes, procDone, time.After(m.shutdownTimeout), &errs)

	// Phase 2: stop services
	svcCancel()
	m.waitPhase(m.services, svcDone, time.After(m.shutdownTimeout), &errs)

	logger.Info(prefix + ": shutdown complete")

	if len(errs) > 0 {
		return fmt.Errorf("%s: %s", prefix, strings.Join(errs, ", "))
	}
	return nil
}

// markStopped records the completion c in entries and logs it.
func (m *Manager) markStopped(entries []entry, c completed) entry {
	entries[c.index].stopped = true
	e := entries[c.index]
	m.logCompleted(e, c.err)
	return e
}

// waitPhase waits for all running entries to complete, or for the deadline.
func (m *Manager) waitPhase(
	entries []entry,
	done <-chan completed,
	deadline <-chan time.Time,
	errs *[]string,
) {
	running := func(e entry) bool { return !e.stopped }
	for slices.ContainsFunc(entries, running) {
		select {
		case c := <-done:
			e := m.markStopped(entries, c)
			m.collectError(errs, e, c.err)
		case <-deadline:
			for _, e := range entries {
				if running(e) {
					logger.Info(m.runnableName() + "/" + e.name + ": still running")
					*errs = append(*errs, fmt.Sprintf("%s is still running", e.name))
				}
			}
			return
		}
	}
}

func (m *Manager) logCompleted(e entry, err error) {
	name := m.runnableName() + "/" + e.name
	if err == nil || errors.Is(err, context.Canceled) {
		logger.Info(name + ": stopped")
	} else {
		logger.Info(name+": stopped with error", "error", err)
	}
}

func (m *Manager) collectError(errs *[]string, e entry, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		*errs = append(*errs, fmt.Sprintf("%s crashed with %+v", e.name, err))
	}
}
