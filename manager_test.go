package runnable

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type mockRunnable struct {
	called     bool
	calledChan chan struct{}

	cancelled     bool
	cancelledChan chan struct{}

	errChan chan error
}

func newMockRunnable() *mockRunnable {
	return &mockRunnable{
		calledChan:    make(chan struct{}),
		cancelledChan: make(chan struct{}),
		errChan:       make(chan error),
	}
}

func (r *mockRunnable) Run(ctx context.Context) error {
	r.called = true
	close(r.calledChan)

	<-ctx.Done()
	r.cancelled = true
	close(r.cancelledChan)

	return <-r.errChan
}

func TestManager_EmptyManager(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		err := NewManager().Run(cancelledContext())
		require.NoError(t, err)
	})
}

func TestManager_Dying_Process(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManager()
		m.RegisterProcess(newDyingRunnable())

		err := m.Run(context.Background())
		require.EqualError(t, err, "manager: dyingRunnable: dying")
	})
}

func TestManager_ShutdownReason(t *testing.T) {
	// shutdownLog runs a manager until the registered runnable returns, and
	// returns the shutdown log line.
	shutdownLog := func(t *testing.T, register func(*Manager)) string {
		t.Helper()
		var line string
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)
			m := NewManager()
			register(m)
			_ = m.Run(context.Background())
			for l := range strings.Lines(logs.String()) {
				if strings.Contains(l, "starting shutdown") {
					line = l
				}
			}
		})
		return line
	}

	t.Run("process returned nil", func(t *testing.T) {
		line := shutdownLog(t, func(m *Manager) { m.RegisterProcess(newCounterRunnable()) })
		require.Equal(t, "level=INFO msg=\"manager: starting shutdown\" reason=\"counter completed\"\n", line)
	})

	t.Run("process returned an error", func(t *testing.T) {
		line := shutdownLog(t, func(m *Manager) { m.RegisterProcess(newDyingRunnable()) })
		require.Equal(t, "level=INFO msg=\"manager: starting shutdown\" reason=\"dyingRunnable died\"\n", line)
	})

	t.Run("service returned nil", func(t *testing.T) {
		line := shutdownLog(t, func(m *Manager) { m.RegisterService(newCounterRunnable()) })
		require.Equal(t, "level=INFO msg=\"manager: starting shutdown\" reason=\"counter completed\"\n", line)
	})

	t.Run("service returned an error", func(t *testing.T) {
		line := shutdownLog(t, func(m *Manager) { m.RegisterService(newDyingRunnable()) })
		require.Equal(t, "level=INFO msg=\"manager: starting shutdown\" reason=\"dyingRunnable died\"\n", line)
	})
}

func TestManager_Dying_Service(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManager()

		proc := newMockRunnable()
		m.RegisterProcess(proc)
		m.RegisterService(newDyingRunnable())

		errChan := make(chan error)
		go func() { errChan <- m.Run(context.Background()) }()

		// Process should be cancelled when service dies.
		<-proc.cancelledChan
		proc.errChan <- nil

		err := <-errChan
		require.EqualError(t, err, "manager: dyingRunnable: dying")
	})
}

func TestManager_ShutdownTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		unblock := make(chan struct{})
		blocked := Named(Func(func(ctx context.Context) error {
			<-unblock
			return nil
		}), "blockedRunnable")

		m := NewManager().ShutdownTimeout(time.Second)
		m.RegisterProcess(blocked)

		err := m.Run(cancelledContext())
		require.EqualError(t, err, "manager: blockedRunnable: still running after shutdown timeout")
		require.ErrorIs(t, err, ErrShutdownTimeout)

		close(unblock) // let the goroutine exit for synctest cleanup
	})
}

func TestManager_ShutdownOrdering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m := NewManager()

		proc := newMockRunnable()
		svc := newMockRunnable()

		m.RegisterProcess(proc)
		m.RegisterService(svc)

		errChan := make(chan error)
		ctx, cancel := context.WithCancel(context.Background())

		go func() { errChan <- m.Run(ctx) }()

		<-proc.calledChan // process has started
		<-svc.calledChan  // service has started

		cancel() // shutdown the manager

		<-proc.cancelledChan // process is cancelled

		synctest.Wait()
		require.False(t, svc.cancelled) // service should NOT be cancelled yet

		proc.errChan <- nil // process shuts down

		<-svc.cancelledChan // service can be cancelled now

		svc.errChan <- nil // service shuts down

		require.NoError(t, <-errChan)
	})
}

func TestManager_Nested(t *testing.T) {
	t.Run("cancellation propagates to inner manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			innerProc := newMockRunnable()

			inner := NewManager()
			inner.RegisterProcess(innerProc)

			outer := NewManager()
			outer.RegisterProcess(Named(inner, "inner"))

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- Named(outer, "outer").Run(ctx) }()

			<-innerProc.calledChan // inner process has started

			cancel() // shutdown the outer manager

			<-innerProc.cancelledChan // inner process is cancelled
			innerProc.errChan <- nil

			require.NoError(t, <-errChan)
		})
	})

	t.Run("inner shutdown ordering preserved", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			innerProc := newMockRunnable()
			innerSvc := newMockRunnable()

			inner := NewManager()
			inner.RegisterProcess(innerProc)
			inner.RegisterService(innerSvc)

			outer := NewManager()
			outer.RegisterProcess(Named(inner, "inner"))

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- Named(outer, "outer").Run(ctx) }()

			<-innerProc.calledChan // inner process has started
			<-innerSvc.calledChan  // inner service has started

			cancel() // shutdown the outer manager

			<-innerProc.cancelledChan // inner process is cancelled first

			synctest.Wait()
			require.False(t, innerSvc.cancelled) // inner service should NOT be cancelled yet

			innerProc.errChan <- nil // inner process shuts down

			<-innerSvc.cancelledChan // inner service can be cancelled now
			innerSvc.errChan <- nil

			require.NoError(t, <-errChan)
		})
	})

	t.Run("inner service outlives outer process", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			innerSvc := newMockRunnable()
			outerProc := newMockRunnable()

			inner := NewManager()
			inner.RegisterService(innerSvc)

			outer := NewManager()
			outer.RegisterProcess(outerProc)
			outer.RegisterService(Named(inner, "inner"))

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- Named(outer, "outer").Run(ctx) }()

			<-outerProc.calledChan // outer process has started
			<-innerSvc.calledChan  // inner service has started

			cancel() // shutdown

			<-outerProc.cancelledChan // outer process cancelled first
			outerProc.errChan <- nil

			// Inner manager (registered as outer service) is cancelled after outer processes.
			<-innerSvc.cancelledChan
			innerSvc.errChan <- nil

			require.NoError(t, <-errChan)
		})
	})
}

func TestManager_DuplicateRegistration(t *testing.T) {
	t.Run("process registered twice", func(t *testing.T) {
		m := NewManager()
		r := newDummyRunnable()
		m.RegisterProcess(r)

		require.PanicsWithValue(t, "runnable dummyRunnable already registered", func() {
			m.RegisterProcess(r)
		})
	})

	t.Run("service registered twice", func(t *testing.T) {
		m := NewManager()
		r := newDummyRunnable()
		m.RegisterService(r)

		require.PanicsWithValue(t, "runnable dummyRunnable already registered", func() {
			m.RegisterService(r)
		})
	})

	t.Run("registered as both process and service", func(t *testing.T) {
		m := NewManager()
		r := newDummyRunnable()
		m.RegisterProcess(r)

		require.PanicsWithValue(t, "runnable dummyRunnable already registered", func() {
			m.RegisterService(r)
		})
	})

	t.Run("registered as service then process", func(t *testing.T) {
		m := NewManager()
		r := newDummyRunnable()
		m.RegisterService(r)

		require.PanicsWithValue(t, "runnable dummyRunnable already registered", func() {
			m.RegisterProcess(r)
		})
	})
}

// mapRunnable is a non-comparable value type: comparing interfaces holding it panics.
type mapRunnable struct {
	_ map[string]int
}

func (r mapRunnable) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestManager_NonComparableRunnable(t *testing.T) {
	t.Run("as process", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			m := NewManager()
			m.RegisterProcess(mapRunnable{}, mapRunnable{})

			require.NoError(t, m.Run(cancelledContext()))
		})
	})

	t.Run("as service", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			m := NewManager()
			m.RegisterService(mapRunnable{}, mapRunnable{})

			require.NoError(t, m.Run(cancelledContext()))
		})
	})

	t.Run("mixed with comparable runnables", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			m := NewManager()
			m.RegisterProcess(newDummyRunnable(), mapRunnable{})
			m.RegisterService(mapRunnable{}, newCounterRunnable())

			require.NoError(t, m.Run(cancelledContext()))
		})
	})
}

func TestManager_RunTwice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		unblock := make(chan struct{})
		blocked := Named(Func(func(ctx context.Context) error {
			<-unblock
			return nil
		}), "blockedRunnable")

		m := NewManager().ShutdownTimeout(time.Second)
		m.RegisterProcess(blocked)

		close(unblock)
		require.NoError(t, m.Run(cancelledContext()))

		// The second run must not consider the runnable stopped from the first run.
		unblock = make(chan struct{})
		err := m.Run(cancelledContext())
		require.EqualError(t, err, "manager: blockedRunnable: still running after shutdown timeout")

		close(unblock)
	})
}

type panickingRunnable struct{}

func (r *panickingRunnable) Run(context.Context) error {
	panic("boom")
}

type failingCloser struct{}

func (c *failingCloser) Close() error { return errors.New("close failed") }

func TestManager_ErrorChain(t *testing.T) {
	t.Run("panic in a process", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			m := NewManager()
			m.RegisterProcess(&panickingRunnable{})

			err := m.Run(context.Background())
			require.EqualError(t, err, "manager: panickingRunnable: runnable panicked: boom")

			var panicErr *PanicError
			require.ErrorAs(t, err, &panicErr)
		})
	})

	t.Run("panic in a nested manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			inner := NewManager()
			inner.RegisterProcess(&panickingRunnable{})

			outer := NewManager()
			outer.RegisterProcess(Named(inner, "inner"))

			err := Named(outer, "outer").Run(context.Background())
			require.EqualError(t, err, "outer: inner: panickingRunnable: runnable panicked: boom")

			var panicErr *PanicError
			require.ErrorAs(t, err, &panicErr)
		})
	})

	t.Run("crash and shutdown timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			unblock := make(chan struct{})
			blocked := Named(Func(func(context.Context) error {
				<-unblock
				return nil
			}), "blockedRunnable")

			m := NewManager().ShutdownTimeout(time.Second)
			m.RegisterProcess(blocked)
			m.RegisterProcess(&panickingRunnable{})

			err := m.Run(context.Background())
			require.EqualError(t, err, "manager: panickingRunnable: runnable panicked: boom\n"+
				"blockedRunnable: still running after shutdown timeout")
			require.ErrorIs(t, err, ErrShutdownTimeout)

			var panicErr *PanicError
			require.ErrorAs(t, err, &panicErr)

			close(unblock) // let the goroutine exit for synctest cleanup
		})
	})

	t.Run("closer error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			m := NewManager()
			m.RegisterService(CloserErr(&failingCloser{}))

			err := m.Run(cancelledContext())
			require.EqualError(t, err, "manager: closer/failingCloser: closer: Close() returned an error: close failed")

			var runnableErr *RunnableError
			require.ErrorAs(t, err, &runnableErr)
		})
	})
}

func TestManager_ShutdownBudget(t *testing.T) {
	// blockUntil returns a runnable that ignores cancellation until unblock is closed.
	blockUntil := func(name string, unblock chan struct{}) Runnable {
		return Named(Func(func(context.Context) error {
			<-unblock
			return nil
		}), name)
	}

	blockOnCancel := func(name string) Runnable {
		return Named(Func(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}), name)
	}

	t.Run("services get what processes left", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			slowProc := Named(Func(func(ctx context.Context) error {
				<-ctx.Done()
				time.Sleep(30 * time.Millisecond)
				return nil
			}), "slowProcess")
			unblock := make(chan struct{})

			m := NewManager().ShutdownTimeout(100 * time.Millisecond)
			m.RegisterProcess(slowProc)
			m.RegisterService(blockUntil("blockedService", unblock))

			start := time.Now()
			err := m.Run(cancelledContext())
			require.Equal(t, "100ms", time.Since(start).String())
			require.EqualError(t, err, "manager: blockedService: still running after shutdown timeout")
			require.ErrorIs(t, err, ErrShutdownTimeout)

			close(unblock) // let the goroutine exit for synctest cleanup
		})
	})

	t.Run("processes get half, services still stop cleanly", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			unblock := make(chan struct{})

			m := NewManager().ShutdownTimeout(100 * time.Millisecond)
			m.RegisterProcess(blockUntil("blockedProcess", unblock))
			m.RegisterService(blockOnCancel("service"))

			start := time.Now()
			err := m.Run(cancelledContext())
			require.Equal(t, "50ms", time.Since(start).String())
			require.EqualError(t, err, "manager: blockedProcess: still running after shutdown timeout")
			require.ErrorIs(t, err, ErrShutdownTimeout)

			close(unblock) // let the goroutine exit for synctest cleanup
		})
	})

	t.Run("both phases time out within the total budget", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			unblock := make(chan struct{})

			m := NewManager().ShutdownTimeout(100 * time.Millisecond)
			m.RegisterProcess(blockUntil("blockedProcess", unblock))
			m.RegisterService(blockUntil("blockedService", unblock))

			start := time.Now()
			err := m.Run(cancelledContext())
			require.Equal(t, "100ms", time.Since(start).String())
			require.EqualError(t, err, "manager: blockedProcess: still running after shutdown timeout\n"+
				"blockedService: still running after shutdown timeout")

			close(unblock) // let the goroutines exit for synctest cleanup
		})
	})
}
