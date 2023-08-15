package runnable

import (
	"context"
	"errors"
	"sync"
)

type StartedRunnable struct {
	baseWrapper

	runnable   Runnable
	shutdownCh chan struct{}
	stoppedCh  chan *StartedRunnable

	mu      sync.Mutex
	running bool
	err     error
}

func StartRunnable(ctx context.Context, r Runnable, shutdownCh chan struct{}, stoppedCh chan *StartedRunnable) *StartedRunnable {
	s := &StartedRunnable{
		baseWrapper: baseWrapper{"", r},
		runnable:    r,
		shutdownCh:  shutdownCh,
		stoppedCh:   stoppedCh,
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

		s.running = false
		s.err = err

		if s.shutdownCh != nil {
			s.shutdownCh <- struct{}{}
		}
		if s.stoppedCh != nil {
			s.stoppedCh <- s
		}
	}()

	return s
}

func (s *StartedRunnable) State() (running bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running, s.err
}
