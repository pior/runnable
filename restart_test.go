package runnable

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRestart(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		r := Restart(newDummyRunnable())
		AssertRunnableRespectCancellation(t, r, time.Millisecond*100)
		AssertRunnableRespectPreCancelledContext(t, r)
	})

	t.Run("restart limit", func(t *testing.T) {
		counter := newCounterRunnable()

		r := Restart(counter, RestartLimit(10))
		err := r.Run(context.Background())
		require.NoError(t, err)

		require.Equal(t, 11, counter.counter) // 10 restarts = 11 executions
	})

	t.Run("errors do not count toward the restart limit", func(t *testing.T) {
		callCount := 0
		fn := Func(func(ctx context.Context) error {
			callCount++
			if callCount <= 3 {
				return errors.New("failed")
			}
			return nil
		})

		r := Restart(fn,
			RestartLimit(1),
			ErrorBackoff(func(int) time.Duration { return 0 }))
		require.NoError(t, r.Run(context.Background()))

		// 3 errors, then a success restarted once.
		require.Equal(t, 5, callCount)
	})

	t.Run("error limit", func(t *testing.T) {
		counter := newDyingRunnable()

		r := Restart(counter,
			RestartErrorLimit(10),
			ErrorBackoff(func(int) time.Duration { return 0 }))
		err := r.Run(context.Background())
		require.EqualError(t, err, "dying")

		require.Equal(t, 10, counter.counter)
	})

	t.Run("error count resets on success", func(t *testing.T) {
		// Alternates: error, success, error, success, ...
		// Error count should never exceed 1, so RestartErrorLimit(2) is never reached.
		callCount := 0
		fn := Func(func(ctx context.Context) error {
			callCount++
			if callCount%2 == 1 {
				return errors.New("odd")
			}
			return nil
		})

		r := Restart(fn,
			RestartErrorLimit(2),
			RestartLimit(3),
			ErrorBackoff(func(int) time.Duration { return 0 }))
		err := r.Run(context.Background())
		require.NoError(t, err) // hit restart limit, not error limit

		// Errors restart without counting toward RestartLimit(3): the 4th success
		// is the 3rd one after a restart, and stops the loop.
		require.Equal(t, 8, callCount)
	})

	t.Run("restarted errors are logged", func(t *testing.T) {
		logs := captureLogs(t)
		r := Restart(newDyingRunnable(), RestartErrorLimit(2), ErrorBackoff(func(int) time.Duration { return 0 }))

		require.EqualError(t, Named("job", r).Run(context.Background()), "dying")
		require.Contains(t, logs.String(),
			`level=INFO msg="job: failed, restarting" error=dying errors=1 delay=0s`+"\n")
	})

	t.Run("restarted panics are logged with the stack once", func(t *testing.T) {
		logs := captureLogs(t)
		r := Restart(&panickingRunnable{}, RestartErrorLimit(2), ErrorBackoff(func(int) time.Duration { return 0 }))
		_ = r.Run(context.Background())

		AssertPanicLogged(t, logs, "failed, restarting")
	})

	t.Run("panic recovery", func(t *testing.T) {
		callCount := 0
		fn := Func(func(ctx context.Context) error {
			callCount++
			panic("boom")
		})

		r := Restart(fn, RestartErrorLimit(3), ErrorBackoff(func(int) time.Duration { return 0 }))
		err := r.Run(context.Background())

		require.Equal(t, 3, callCount)
		var panicErr *PanicError
		require.ErrorAs(t, err, &panicErr)
		require.Equal(t, "runnable panicked: boom", err.Error())
	})

	t.Run("error backoff", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var count atomic.Int32
			fn := Func(func(ctx context.Context) error {
				count.Add(1)
				return errors.New("fail")
			})

			ctx, cancel := context.WithCancel(context.Background())

			r := Restart(fn, ErrorBackoff(func(n int) time.Duration {
				return 10 * time.Second
			}))

			errChan := make(chan error, 1)
			go func() {
				errChan <- r.Run(ctx)
			}()

			// First run is immediate. Then 10s backoff before each retry.
			time.Sleep(1 * time.Nanosecond) // let first run complete
			require.Equal(t, int32(1), count.Load())

			time.Sleep(10*time.Second + 1)
			require.Equal(t, int32(2), count.Load())

			time.Sleep(10*time.Second + 1)
			require.Equal(t, int32(3), count.Load())

			cancel()
			err := <-errChan
			require.EqualError(t, err, "context canceled")
		})
	})

	t.Run("error reset after stable run", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var runCount atomic.Int32
			fn := Func(func(ctx context.Context) error {
				// Simulate a service that runs for a while then fails.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Hour):
					runCount.Add(1)
					return errors.New("fail")
				}
			})

			ctx, cancel := context.WithCancel(context.Background())

			backoffCalls := []int{}
			r := Restart(fn,
				ErrorResetAfter(30*time.Minute),
				ErrorBackoff(func(n int) time.Duration {
					backoffCalls = append(backoffCalls, n)
					return 0
				}))

			errChan := make(chan error, 1)
			go func() {
				errChan <- r.Run(ctx)
			}()

			// First run: runs for 1h (≥30m reset threshold), then errors.
			// Error count resets before incrementing → errorCount=1.
			time.Sleep(time.Hour + 1*time.Nanosecond)
			require.Equal(t, int32(1), runCount.Load())

			// Second run: same pattern. Error count resets again → errorCount=1.
			time.Sleep(time.Hour + 1*time.Nanosecond)
			require.Equal(t, int32(2), runCount.Load())

			cancel()
			<-errChan

			// Both backoff calls received errorCount=1 because each run
			// lasted ≥ ErrorResetAfter, resetting the count before incrementing.
			// Without ErrorResetAfter, the second call would have received 2.
			require.Equal(t, []int{1, 1}, backoffCalls)
		})
	})
}
