package runnable

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newDummyRunnable() *dummyRunnable {
	return &dummyRunnable{}
}

type dummyRunnable struct{}

func (*dummyRunnable) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func newDyingRunnable() *dyingRunnable {
	return &dyingRunnable{}
}

type dyingRunnable struct{}

func (*dyingRunnable) Run(ctx context.Context) error {
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

func AssertName(t *testing.T, expectedName string, runnable Runnable) {
	name := runnable.(interface{ name() string }).name()
	require.Equal(t, expectedName, name)
}

func AssertRunnableRespectCancellation(t *testing.T, runnable Runnable, waitTime time.Duration) {
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
		require.NoError(t, err)
	}
}

func AssertRunnableRespectPreCancelledContext(t *testing.T, runnable Runnable) {
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
		require.NoError(t, err)
	}
}

func AssertTimeout(t *testing.T, waitTime time.Duration, fn func()) {
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
