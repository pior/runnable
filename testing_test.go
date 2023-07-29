package runnable

import (
	"context"
	"errors"
	stdlog "log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func funcTesting(context.Context) error { return nil }

func newDummyRunnable() *dummyRunnable {
	return &dummyRunnable{}
}

type dummyRunnable struct{}

func (r *dummyRunnable) Run(ctx context.Context) error {
	Log(r, "started")
	<-ctx.Done()
	Log(r, "stopped")
	return ctx.Err()
}

func newCounterRunnable() *counter {
	return &counter{}
}

type counter struct {
	counter int
}

func (c *counter) Run(ctx context.Context) error {
	c.counter++
	return nil
}

func newDyingRunnable() *dyingRunnable {
	return &dyingRunnable{}
}

type dyingRunnable struct {
	counter int
}

func (r *dyingRunnable) Run(ctx context.Context) error {
	r.counter++
	return errors.New("dying")
}

func newBlockedRunnable() *blockedRunnable {
	return &blockedRunnable{}
}

type blockedRunnable struct{}

func (*blockedRunnable) Run(ctx context.Context) error {
	select {}
}

type dummyError struct {
	message string
}

func (e *dummyError) Error() string {
	return e.message
}

func AssertRunnableRespectCancellation(t *testing.T, runnable Runnable, waitTime time.Duration) {
	t.Helper()

	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error)

	go func() {
		errChan <- runnable.Run(ctx)
	}()

	cancelFunc()

	select {
	case <-time.After(waitTime):
		t.Fatal("did not return after " + waitTime.String())
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
	}
}

func AssertRunnableRespectPreCancelledContext(t *testing.T, runnable Runnable) {
	t.Helper()

	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	errChan := make(chan error)

	go func() {
		errChan <- runnable.Run(ctx)
	}()

	select {
	case <-time.After(time.Millisecond * 100):
		t.Fatal("did not return after 100ms")
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
	}
}

func AssertTimeout(t *testing.T, waitTime time.Duration, fn func()) {
	t.Helper()

	wait := make(chan bool)

	go func() {
		fn()
		close(wait)
	}()

	select {
	case <-time.After(waitTime):
		t.Fatal("timeout: " + waitTime.String())
	case <-wait:
	}
}

func cancelledContext() context.Context {
	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()
	return ctx
}

func initializeForExample() (context.Context, func()) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)

	SetLogger(stdlog.New(os.Stdout, "", 0))

	return ctx, cancel
}
