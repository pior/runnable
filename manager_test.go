package runnable

import (
	"context"
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
		require.EqualError(t, err, "manager: dyingRunnable crashed with dying")
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
		require.EqualError(t, err, "manager: dyingRunnable crashed with dying")
	})
}

func TestManager_ShutdownTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		unblock := make(chan struct{})
		blocked := Func(func(ctx context.Context) error {
			<-unblock
			return nil
		}).Name("blockedRunnable")

		m := NewManager().ShutdownTimeout(time.Second)
		m.RegisterProcess(blocked)

		err := m.Run(cancelledContext())
		require.EqualError(t, err, "manager: blockedRunnable is still running")

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

			inner := NewManager().Name("inner")
			inner.RegisterProcess(innerProc)

			outer := NewManager().Name("outer")
			outer.RegisterProcess(inner)

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- outer.Run(ctx) }()

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

			inner := NewManager().Name("inner")
			inner.RegisterProcess(innerProc)
			inner.RegisterService(innerSvc)

			outer := NewManager().Name("outer")
			outer.RegisterProcess(inner)

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- outer.Run(ctx) }()

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

			inner := NewManager().Name("inner")
			inner.RegisterService(innerSvc)

			outer := NewManager().Name("outer")
			outer.RegisterProcess(outerProc)
			outer.RegisterService(inner)

			errChan := make(chan error)
			ctx, cancel := context.WithCancel(context.Background())

			go func() { errChan <- outer.Run(ctx) }()

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
		blocked := Func(func(ctx context.Context) error {
			<-unblock
			return nil
		}).Name("blockedRunnable")

		m := NewManager().ShutdownTimeout(time.Second)
		m.RegisterProcess(blocked)

		close(unblock)
		require.NoError(t, m.Run(cancelledContext()))

		// The second run must not consider the runnable stopped from the first run.
		unblock = make(chan struct{})
		err := m.Run(cancelledContext())
		require.EqualError(t, err, "manager: blockedRunnable is still running")

		close(unblock)
	})
}
