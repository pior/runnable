package runnable

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeout(t *testing.T) {
	// blocking returns a runnable that blocks until cancelled, then returns err.
	blocking := func(err error) Runnable {
		return Func(func(ctx context.Context) error {
			<-ctx.Done()
			return err
		})
	}
	noop := Func(func(context.Context) error { return nil })
	returnsCtxErr := Func(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	t.Run("returns the result when it finishes in time", func(t *testing.T) {
		logs := captureLogs(t)
		failing := Func(func(context.Context) error { return errors.New("failed") })

		require.NoError(t, Timeout(time.Second, noop).Run(context.Background()))
		require.EqualError(t, Timeout(time.Second, failing).Run(context.Background()), "failed")
		require.Empty(t, logs.String())
	})

	t.Run("times out", func(t *testing.T) {
		logs := captureLogs(t)

		err := Named("job", Timeout(10*time.Millisecond, returnsCtxErr)).Run(context.Background())
		require.EqualError(t, err, "timed out after 10ms: context deadline exceeded")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, `level=INFO msg="job: timed out" timeout=10ms`+"\n", logs.String())
	})

	t.Run("succeeds when the runnable stops cleanly on the timeout", func(t *testing.T) {
		logs := captureLogs(t)

		require.NoError(t, Named("job", Timeout(10*time.Millisecond, blocking(nil))).Run(context.Background()))
		require.Equal(t, `level=INFO msg="job: timed out" timeout=10ms`+"\n", logs.String())
	})

	t.Run("returns another error of a timed out runnable as is", func(t *testing.T) {
		r := Func(func(ctx context.Context) error {
			<-ctx.Done()
			return fmt.Errorf("query: %w", ctx.Err())
		})

		err := Timeout(10*time.Millisecond, r).Run(context.Background())
		require.EqualError(t, err, "query: context deadline exceeded")
	})

	t.Run("parent cancellation is not a timeout", func(t *testing.T) {
		logs := captureLogs(t)

		err := Timeout(time.Minute, returnsCtxErr).Run(cancelledContext())
		require.ErrorIs(t, err, context.Canceled)
		require.NoError(t, Timeout(time.Minute, blocking(nil)).Run(cancelledContext()))
		require.Empty(t, logs.String())
	})

	t.Run("parent deadline is not a timeout", func(t *testing.T) {
		logs := captureLogs(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		t.Cleanup(cancel)

		err := Timeout(time.Minute, returnsCtxErr).Run(ctx)
		require.Equal(t, context.DeadlineExceeded, err)
		require.Empty(t, logs.String())
	})

	t.Run("name", func(t *testing.T) {
		require.Equal(t, "timeout/dummyRunnable", runnableName(Timeout(time.Second, newDummyRunnable())))
	})
}
