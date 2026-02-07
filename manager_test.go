package runnable

import (
	"context"
	"testing"
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

func TestManager_Cancellation(t *testing.T) {
	m := Manager()
	m.Add(newDummyRunnable())
	AssertRunnableRespectCancellation(t, m, time.Millisecond*100)
	AssertRunnableRespectPreCancelledContext(t, Manager())
}

func TestManager_Without_Runnable(t *testing.T) {
	m := Manager()
	AssertRunnableRespectCancellation(t, m, time.Millisecond*100)
}

func TestManager_Dying_Process(t *testing.T) {
	m := Manager()
	m.Add(newDyingRunnable())

	AssertTimeout(t, time.Second*1, func() {
		err := m.Run(context.Background())
		require.EqualError(t, err, "manager: dyingRunnable crashed with dying")
	})
}

func TestManager_Dying_Service(t *testing.T) {
	m := Manager()

	proc := newMockRunnable()
	m.Add(proc)
	m.AddService(newDyingRunnable())

	errChan := make(chan error)
	go func() {
		errChan <- m.Run(context.Background())
	}()

	// Process should be cancelled when service dies.
	<-proc.cancelledChan
	proc.errChan <- nil

	err := <-errChan
	require.EqualError(t, err, "manager: dyingRunnable crashed with dying")
}

func TestManager_ShutdownTimeout(t *testing.T) {
	m := Manager().ShutdownTimeout(time.Second)
	m.Add(newBlockedRunnable())

	ctx := cancelledContext()

	AssertTimeout(t, time.Second*2, func() {
		err := m.Run(ctx)
		require.EqualError(t, err, "manager: blockedRunnable is still running")
	})
}

func TestManager_ShutdownOrdering(t *testing.T) {
	m := Manager()

	proc := newMockRunnable()
	svc := newMockRunnable()

	m.Add(proc)
	m.AddService(svc)

	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		errChan <- m.Run(ctx)
	}()

	<-proc.calledChan // process has started
	<-svc.calledChan  // service has started

	cancel() // shutdown the manager

	<-proc.cancelledChan // process is cancelled

	time.Sleep(time.Millisecond * 100)
	require.False(t, svc.cancelled) // service should NOT be cancelled yet

	proc.errChan <- nil // process shuts down

	<-svc.cancelledChan // service can be cancelled now

	svc.errChan <- nil // service shuts down

	require.NoError(t, <-errChan)
}
