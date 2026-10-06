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

	t.Run("error limit", func(t *testing.T) {
		logs := captureLogs(t)
		dying := newDyingRunnable()

		err := Named("job", Retry(dying, ErrorLimit(3), noBackoff)).Run(context.Background())
		require.EqualError(t, err, "dying")
		require.Equal(t, 3, dying.counter)
		require.Contains(t, logs.String(), `level=INFO msg="job: not retrying" reason="error limit" limit=3`+"\n")
	})

	t.Run("succeeds on the last run before the error limit", func(t *testing.T) {
		r, calls := failingTimes(2)

		require.NoError(t, Retry(r, ErrorLimit(3), noBackoff).Run(context.Background()))
		require.Equal(t, 3, *calls)
	})

	t.Run("error count resets after a long run", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			// Fails 3 times: quickly, after a long run, then quickly.
			calls := 0
			r := Func(func(context.Context) error {
				calls++
				if calls == 2 {
					time.Sleep(time.Hour)
				}
				if calls <= 3 {
					return errors.New("failed")
				}
				return nil
			})

			var counts []int
			backoff := ErrorBackoff(func(n int) time.Duration {
				counts = append(counts, n)
				return 0
			})

			// Without the reset, the third error would reach the limit.
			err := Retry(r, ErrorLimit(3), ErrorResetAfter(30*time.Minute), backoff).Run(context.Background())
			require.NoError(t, err)
			require.Equal(t, "[1 1 2]", fmt.Sprint(counts))
		})
	})

	t.Run("retried panics are logged with the stack once", func(t *testing.T) {
		logs := captureLogs(t)
		_ = Retry(&panickingRunnable{}, ErrorLimit(2), noBackoff).Run(context.Background())

		AssertPanicLogged(t, logs, "failed, retrying")
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

	t.Run("on error", func(t *testing.T) {
		// errorsSeen returns an OnError option that records the errors.
		errorsSeen := func() (RetryOption, *[]string) {
			var seen []string
			return OnError(func(err error) { seen = append(seen, err.Error()) }), &seen
		}

		t.Run("called for each retried error", func(t *testing.T) {
			r, _ := failingTimes(2)
			onError, seen := errorsSeen()

			require.NoError(t, Retry(r, onError, noBackoff).Run(context.Background()))
			require.Equal(t, "[failed failed]", fmt.Sprint(*seen))
		})

		t.Run("called for the last error at the error limit", func(t *testing.T) {
			onError, seen := errorsSeen()

			err := Retry(newDyingRunnable(), onError, ErrorLimit(3), noBackoff).Run(context.Background())
			require.EqualError(t, err, "dying")
			require.Equal(t, "[dying dying dying]", fmt.Sprint(*seen))
		})

		t.Run("receives a recovered panic", func(t *testing.T) {
			var panicErr *PanicError
			onError := OnError(func(err error) { require.ErrorAs(t, err, &panicErr) })

			_ = Retry(&panickingRunnable{}, onError, ErrorLimit(1)).Run(context.Background())
			require.NotNil(t, panicErr)
		})

		t.Run("not called on cancellation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			r := Func(func(context.Context) error {
				cancel()
				return errors.New("failed")
			})
			onError, seen := errorsSeen()

			require.ErrorIs(t, Retry(r, onError).Run(ctx), context.Canceled)
			require.Empty(t, *seen)
		})
	})

	t.Run("name", func(t *testing.T) {
		require.Equal(t, "retry/dummyRunnable", runnableName(Retry(newDummyRunnable())))
	})
}
