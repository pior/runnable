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
	g := NewManager()
	g.Add(newDummyRunnable())
	AssertRunnableRespectCancellation(t, g.Build(), time.Millisecond*100)
	AssertRunnableRespectPreCancelledContext(t, g.Build())
}

func TestManager_Without_Runnable(t *testing.T) {
	g := NewManager()
	AssertRunnableRespectCancellation(t, g.Build(), time.Millisecond*100)
}

func TestManager_Dying_Runnable(t *testing.T) {
	g := NewManager()
	g.Add(newDyingRunnable())

	AssertTimeout(t, time.Second*1, func() {
		err := g.Build().Run(context.Background())
		require.EqualError(t, err, "manager: runnable.dyingRunnable crashed with dying")
	})
}

func TestManager_ShutdownTimeout(t *testing.T) {
	g := NewManager(ManagerShutdownTimeout(time.Second))
	g.Add(newBlockedRunnable())

	ctx := cancelledContext()

	AssertTimeout(t, time.Second*2, func() {
		err := g.Build().Run(ctx)
		require.EqualError(t, err, "manager: runnable.blockedRunnable is still running")
	})

}

func TestManager(t *testing.T) {
	g := NewManager()

	web := newMockRunnable()
	db := newMockRunnable()

	g.Add(db)
	g.Add(web, db)

	errChan := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		errChan <- g.Build().Run(ctx)
	}()

	<-web.calledChan // "web" has started
	<-db.calledChan  // "db" has started

	cancel() // shutdown the manager

	<-web.cancelledChan // "web" is cancelled

	time.Sleep(time.Millisecond * 100)
	require.False(t, db.cancelled) // "db" should not be shutdown yet

	web.errChan <- nil // "web" shuts down

	<-db.cancelledChan // "db" can be cancelled now

	db.errChan <- nil // "db" shuts down

	require.NoError(t, <-errChan)
}
