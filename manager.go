package runnable

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type manager struct {
	containers []*managerContainer
	options    ManagerOptions
}

func (m *manager) log(format string, args ...interface{}) {
	log.Infof("manager: "+format, args...)
}

func (m *manager) Run(ctx context.Context) error {
	dying := make(chan *managerContainer, len(m.containers))
	completedChan := make(chan *managerContainer, len(m.containers))

	// run the runnables in Go routines.
	for _, c := range m.containers {
		c.launch(completedChan, dying)
		m.log("%s started", c.name())
	}

	// block until group is cancelled, or a runnable dies.
	select {
	case <-ctx.Done():
		m.log("starting shutdown (context cancelled)")
	case c := <-dying:
		m.log("starting shutdown (%s died)", c.name())
	}

	// starting shutdown
	cancelled := newManagerContainerSet()
	completed := newManagerContainerSet()

	deadline := time.After(m.options.ShutdownTimeout)
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
				m.log("%s cancelled", c.name())
				c.shutdown()
				cancelled.insert(c)
			}
		}

		// record terminated runners, quit when complete
		select {
		case c := <-completedChan:
			completed.insert(c)

			if c.err == nil || errors.Is(c.err, context.Canceled) {
				m.log("%s stopped", c.name())
			} else {
				m.log("%s stopped with error: %+v", c.name(), c.err)
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
			m.log("%s is still running", c.name())
			errs = append(errs, fmt.Sprintf("%s is still running", c.name()))
		}
		if c.err != nil && c.err != context.Canceled {
			errs = append(errs, fmt.Sprintf("%s crashed with %+v", c.name(), c.err))
		}
	}

	m.log("shutdown complete")

	if len(errs) != 0 {
		return fmt.Errorf("manager: %s", strings.Join(errs, ", "))
	}
	return nil
}
