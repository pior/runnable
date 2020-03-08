package runnable

import (
	"time"
)

// Manager returns a runnable that execute runnables in go routines.
// Runnables can declare a dependency on another runnable. Dependencies are started first and stopped last.
func Manager(options *ManagerOptions) *managerBuilder {
	if options == nil {
		options = &ManagerOptions{
			ShutdownTimeout: time.Second * 10,
		}
	}
	return &managerBuilder{options: *options}
}

// ManagerOptions configures the behavior of a Manager.
type ManagerOptions struct {
	ShutdownTimeout time.Duration
}

type managerBuilder struct {
	containers []*managerContainer
	options    ManagerOptions
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
	return &manager{m.containers, m.options}
}
