package runnable

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
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
// collected, except [context.Canceled] which is ignored. Run returns them joined with
// [errors.Join], each wrapped with the runnable name, so they can be inspected with
// [errors.Is] and [errors.As]. A runnable still running when the shutdown timeout
// expires is reported with [ErrShutdownTimeout]. A manager is itself a [Runnable],
// so managers can be nested for independent shutdown ordering.
//
// Each runnable runs with its full name in the context, such as "manager/JobQueue",
// see [NameFromContext]. A nested manager uses its assigned name as prefix, and
// leaves the error prefix to its parent.
//
// Registering the same runnable twice, or as both a process and a service, panics.
type Manager struct {
	processes       []entry
	services        []entry
	shutdownTimeout time.Duration
}

// NewManager returns a new [Manager]. Its name, used as a prefix in log messages
// and errors, is "manager" unless a parent [Manager] or [Named] assigns one.
func NewManager() *Manager {
	return &Manager{shutdownTimeout: 10 * time.Second}
}

func (m *Manager) runnableName() string { return "manager" }

// ShutdownTimeout sets the total time for both shutdown phases. Processes get
// half of it, services get the rest: at least half, more when processes stop
// early. Defaults to 10 seconds.
//
// It maps to a platform grace period such as Kubernetes
// terminationGracePeriodSeconds, which must exceed this value to leave room for
// the process to exit. For nested managers, the inner timeout must be smaller
// than the outer one.
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
	parent := nameFromContext(ctx)
	prefix := resolveName(ctx, m.runnableName())
	childName := func(e entry) string { return prefix + "/" + e.name }

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
			svcDone <- completed{i, Recover(svc.runnable).Run(withManagerName(svcCtx, childName(svc)))}
		}()
		logger.Info(childName(svc) + ": started")
	}

	for i, proc := range m.processes {
		go func() {
			procDone <- completed{i, Recover(proc.runnable).Run(withManagerName(procCtx, childName(proc)))}
		}()
		logger.Info(childName(proc) + ": started")
	}

	var errs []error

	// Wait for context cancellation or any runnable to complete.
	select {
	case <-ctx.Done():
		logger.Info(prefix+": starting shutdown", "reason", "context cancelled")
	case c := <-procDone:
		e := markStopped(prefix, m.processes, c)
		collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", completionReason(e, c.err))
	case c := <-svcDone:
		e := markStopped(prefix, m.services, c)
		collectError(&errs, e, c.err)
		logger.Info(prefix+": starting shutdown", "reason", completionReason(e, c.err))
	}

	// One budget for both phases: processes get half, services get the rest.
	deadline, cancelDeadline := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancelDeadline()
	procDeadline, cancelProcDeadline := context.WithTimeout(deadline, m.shutdownTimeout/2)
	defer cancelProcDeadline()

	// Phase 1: stop processes
	procCancel()
	waitPhase(prefix, m.processes, procDone, procDeadline.Done(), &errs)

	// Phase 2: stop services
	svcCancel()
	waitPhase(prefix, m.services, svcDone, deadline.Done(), &errs)

	logger.Info(prefix + ": shutdown complete")

	if len(errs) == 0 {
		return nil
	}
	// A parent manager already wraps this error with our name.
	if parent.fromManager {
		return errors.Join(errs...)
	}
	return fmt.Errorf("%s: %w", prefix, errors.Join(errs...))
}

// markStopped records the completion c in entries and logs it.
func markStopped(prefix string, entries []entry, c completed) entry {
	entries[c.index].stopped = true
	e := entries[c.index]
	logCompleted(prefix+"/"+e.name, c.err)
	return e
}

// waitPhase waits for all running entries to complete, or for the deadline.
func waitPhase(
	prefix string,
	entries []entry,
	done <-chan completed,
	deadline <-chan struct{},
	errs *[]error,
) {
	running := func(e entry) bool { return !e.stopped }
	for slices.ContainsFunc(entries, running) {
		select {
		case c := <-done:
			e := markStopped(prefix, entries, c)
			collectError(errs, e, c.err)
		case <-deadline:
			for _, e := range entries {
				if running(e) {
					logger.Info(prefix + "/" + e.name + ": still running")
					*errs = append(*errs, fmt.Errorf("%s: %w", e.name, ErrShutdownTimeout))
				}
			}
			return
		}
	}
}

// completionReason describes why the entry that triggered the shutdown stopped.
func completionReason(e entry, err error) string {
	if err == nil {
		return e.name + " completed"
	}
	return e.name + " died"
}

func logCompleted(name string, err error) {
	var pe *PanicError
	switch {
	case err == nil || errors.Is(err, context.Canceled):
		logger.Info(name + ": stopped")
	case errors.As(err, &pe):
		// slog's TextHandler formats errors with %+v, which for a PanicError
		// includes the stack. Pass the message as a string to log the stack once.
		logger.Info(name+": stopped with error", "error", err.Error(), "stack", string(pe.Stack))
	default:
		logger.Info(name+": stopped with error", "error", err)
	}
}

func collectError(errs *[]error, e entry, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		*errs = append(*errs, fmt.Errorf("%s: %w", e.name, err))
	}
}
