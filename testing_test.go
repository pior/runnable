package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func AssertRunnableRespectCancellation(t *testing.T, runnable Runnable) {
	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error)

	go func() {
		errChan <- runnable.Run(ctx)
	}()

	cancelFunc()

	select {
	case <-time.After(time.Millisecond * 100):
		t.Fatal("did not return after 100ms")
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
