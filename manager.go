package runnable

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AppManager interface {
	Add(runnable Runnable, dependencies ...Runnable)
	Build() Runnable
}

// ManagerOption configures the behavior of a Manager.
type ManagerOption func(*manager)

func ManagerName(name string) ManagerOption {
	return func(m *manager) {
		m.name = name
	}
}

func ManagerShutdownTimeout(dur time.Duration) ManagerOption {
	return func(m *manager) {
		m.shutdownTimeout = dur
	}
}

// NewManager returns a runnable that execute runnables in go routines.
// Runnables can declare a dependency on another runnable. Dependencies are started first and stopped last.
func NewManager(opts ...ManagerOption) AppManager {
	m := &manager{
		name:            "manager",
		shutdownTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(m)
	}

	return m
}

type manager struct {
	name            string
	containers      []*managerContainer
	shutdownTimeout time.Duration
}

func (m *manager) runnableName() string { return m.name }

func (m *manager) Add(runnable Runnable, dependencies ...Runnable) {
	container := m.insertRunnable(runnable)
	for _, dep := range dependencies {
		m.insertRunnable(dep).insertUser(container)
	}
}

func (m *manager) findRunnable(runnable Runnable) *managerContainer {
	for _, container := range m.containers {
		if container.runnable == runnable {
			return container
		}
	}
	return nil
}

func (m *manager) insertRunnable(runnable Runnable) (value *managerContainer) {
	value = m.findRunnable(runnable)
	if value == nil {
		value = newManagerContainer(runnable)
		m.containers = append(m.containers, value)
	}
	return value
}

func (m *manager) Build() Runnable {
	return m
}

func (m *manager) Run(ctx context.Context) error {
	dying := make(chan *managerContainer, len(m.containers))
	completedChan := make(chan *managerContainer, len(m.containers))

	// run the runnables in Go routines.
	for _, c := range m.containers {
		c.launch(completedChan, dying)
		logger.Info("started", "runnable", m.runnableName()+"/"+c.name())
	}

	// block until group is cancelled, or a runnable dies.
	select {
	case <-ctx.Done():
		logger.Info("starting shutdown", "runnable", m.runnableName(), "reason", "context cancelled")
	case c := <-dying:
		logger.Info("starting shutdown", "runnable", m.runnableName(), "reason", c.name()+" died")
	}

	// starting shutdown
	cancelled := newManagerContainerSet()
	completed := newManagerContainerSet()

	deadline := time.After(m.shutdownTimeout)
	ticker := time.NewTicker(time.Millisecond * 10)
	defer ticker.Stop()

	shutdown := false

	for !shutdown {
		if len(m.containers) == 0 {
			break
		}

		// shutdown runners one by one, if possible (not a dependency of another runner)
		for _, c := range m.containers {
			if completed.contains(c) {
				continue
			}

			if completed.containerHasRunningUsers(c) {
				continue
			}

			if !cancelled.contains(c) {
				logger.Info("cancelled", "runnable", m.runnableName()+"/"+c.name())
				c.shutdown()
				cancelled.insert(c)
			}
		}

		// record terminated runners, quit when complete
		select {
		case c := <-completedChan:
			completed.insert(c)

			if c.err == nil || errors.Is(c.err, context.Canceled) {
				logger.Info("stopped", "runnable", m.runnableName()+"/"+c.name())
			} else {
				logger.Info("stopped with error", "runnable", m.runnableName()+"/"+c.name(), "error", c.err)
			}

			if len(completed) == len(m.containers) {
				shutdown = true
			}
		case <-deadline:
			shutdown = true
		case <-ticker.C:
		}
	}

	errs := []string{}
	for _, c := range m.containers {
		if !completed.contains(c) {
			logger.Info("still running", "runnable", m.runnableName()+"/"+c.name())
			errs = append(errs, fmt.Sprintf("%s is still running", c.name()))
		}
		if c.err != nil && !errors.Is(c.err, context.Canceled) {
			errs = append(errs, fmt.Sprintf("%s crashed with %+v", c.name(), c.err))
		}
	}

	logger.Info("shutdown complete", "runnable", m.runnableName())

	if len(errs) != 0 {
		return fmt.Errorf("manager: %s", strings.Join(errs, ", "))
	}
	return nil
}
