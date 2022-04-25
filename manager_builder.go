package runnable

import (
	"time"
)

// Manager returns a runnable that execute runnables in go routines.
// Runnables can declare a dependency on another runnable. Dependencies are started first and stopped last.
func Manager(opts ...ManagerOption) *managerBuilder {
	m := &managerBuilder{
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

// ManagerOption configures the behavior of a Manager.
type ManagerOption func(*managerBuilder)

func ManagerShutdownTimeout(dur time.Duration) ManagerOption {
	return func(m *managerBuilder) {
		m.shutdownTimeout = dur
	}
}

type managerBuilder struct {
	containers      []*managerContainer
	shutdownTimeout time.Duration
}

func (m *managerBuilder) Add(runnable Runnable, dependencies ...Runnable) {
	container := m.insertRunnable(runnable)
	for _, dep := range dependencies {
		m.insertRunnable(dep).insertUser(container)
	}
}

func (m *managerBuilder) findRunnable(runnable Runnable) *managerContainer {
	for _, container := range m.containers {
		if container.runnable == runnable {
			return container
		}
	}
	return nil
}

func (m *managerBuilder) insertRunnable(runnable Runnable) (value *managerContainer) {
	value = m.findRunnable(runnable)
	if value == nil {
		value = newManagerContainer(runnable)
		m.containers = append(m.containers, value)
	}
	return value
}

func (m *managerBuilder) Build() Runnable {
	return &manager{m.containers, m.shutdownTimeout}
}
