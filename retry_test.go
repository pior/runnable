package runnable

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	noBackoff := ErrorBackoff(func(int) time.Duration { return 0 })

	// failingTimes returns a runnable that fails n times, then succeeds.
	failingTimes := func(n int) (Runnable, *int) {
		calls := 0
		return Func(func(context.Context) error {
			calls++
			if calls <= n {
				return errors.New("failed")
			}
			return nil
		}), &calls
	}

	t.Run("cancellation", func(t *testing.T) {
		r := Retry(newDummyRunnable())
		AssertRunnableRespectCancellation(t, r, time.Millisecond*100)
		AssertRunnableRespectPreCancelledContext(t, r)
	})

	t.Run("success is not retried", func(t *testing.T) {
		counter := newCounterRunnable()

		require.NoError(t, Retry(counter).Run(context.Background()))
		require.Equal(t, 1, counter.counter)
	})

	t.Run("retries until success", func(t *testing.T) {
		logs := captureLogs(t)
		r, calls := failingTimes(2)

		require.NoError(t, Named("job", Retry(r, noBackoff)).Run(context.Background()))
		require.Equal(t, 3, *calls)
		require.Equal(t, `level=INFO msg="job: failed, retrying" error=failed errors=1 delay=0s`+"\n"+
			`level=INFO msg="job: failed, retrying" error=failed errors=2 delay=0s`+"\n", logs.String())
	})

	t.Run("retry limit", func(t *testing.T) {
		logs := captureLogs(t)
		dying := newDyingRunnable()

		err := Named("job", Retry(dying, RetryLimit(3), noBackoff)).Run(context.Background())
		require.EqualError(t, err, "dying")
		require.Equal(t, 4, dying.counter) // 3 retries = 4 executions
		require.Contains(t, logs.String(), `level=INFO msg="job: not retrying" reason="retry limit" limit=3`+"\n")
	})

	t.Run("succeeds on the last retry", func(t *testing.T) {
		r, calls := failingTimes(3)

		require.NoError(t, Retry(r, RetryLimit(3), noBackoff).Run(context.Background()))
		require.Equal(t, 4, *calls)
	})

	t.Run("panic recovery", func(t *testing.T) {
		calls := 0
		fn := Func(func(context.Context) error {
			calls++
			if calls == 1 {
				panic("boom")
			}
			return nil
		})

		require.NoError(t, Retry(fn, noBackoff).Run(context.Background()))
		require.Equal(t, 2, calls)
	})

	t.Run("error backoff", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var counts []int
			backoff := ErrorBackoff(func(n int) time.Duration {
				counts = append(counts, n)
				return 10 * time.Second
			})
			r, calls := failingTimes(2)

			start := time.Now()
			require.NoError(t, Retry(r, backoff).Run(context.Background()))
			require.Equal(t, 3, *calls)
			require.Equal(t, "[1 2]", fmt.Sprint(counts))
			require.Equal(t, "20s", time.Since(start).String())
		})
	})

	t.Run("cancelled during backoff", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(5*time.Second, cancel)

			err := Retry(newDyingRunnable(), ErrorBackoff(func(int) time.Duration { return time.Minute })).Run(ctx)
			require.ErrorIs(t, err, context.Canceled)
		})
	})

	t.Run("name", func(t *testing.T) {
		require.Equal(t, "retry/dummyRunnable", runnableName(Retry(newDummyRunnable())))
	})
}
