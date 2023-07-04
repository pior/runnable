package runnable

import (
	"context"
	"errors"
	"sync"
)

type StartedRunnable struct {
	baseWrapper

	runnable   Runnable
	stoppedChs []chan *StartedRunnable

	mu      sync.Mutex
	stopped bool
	err     error
}

func StartRunnable(ctx context.Context, r Runnable, stoppedChs ...chan *StartedRunnable) *StartedRunnable {
	s := &StartedRunnable{
		baseWrapper: baseWrapper{"", r},
		runnable:    r,
		stoppedChs:  stoppedChs,
	}

	go func() {
		Log(r, "started")
		err := Recover(r).Run(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			Log(r, "stopped")
		} else {
			Log(r, "stopped with error: %+v", err)
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		s.stopped = false
		s.err = err

		for _, stoppedCh := range stoppedChs {
			stoppedCh <- s
		}
	}()

	return s
}

func (s *StartedRunnable) State() (stopped bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped, s.err
}
