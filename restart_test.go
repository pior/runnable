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

		r := Restart(counter).Limit(10)
		err := r.Run(context.Background())
		require.NoError(t, err)

		require.Equal(t, 11, counter.counter) // 10 restarts = 11 executions
	})

	t.Run("error limit", func(t *testing.T) {
		counter := newDyingRunnable()

		r := Restart(counter).
			ErrorLimit(10).
			ErrorBackoff(func(int) time.Duration { return 0 })
		err := r.Run(context.Background())
		require.EqualError(t, err, "dying")

		require.Equal(t, 10, counter.counter)
	})

	t.Run("error count resets on success", func(t *testing.T) {
		// Alternates: error, success, error, success, ...
		// Error count should never exceed 1, so ErrorLimit(2) is never reached.
		callCount := 0
		fn := Func(func(ctx context.Context) error {
			callCount++
			if callCount%2 == 1 {
				return errors.New("odd")
			}
			return nil
		})

		r := Restart(fn).
			ErrorLimit(2).
			Limit(3).
			ErrorBackoff(func(int) time.Duration { return 0 })
		err := r.Run(context.Background())
		require.NoError(t, err) // hit restart limit, not error limit

		// Sequence: run1=err(errorCount=1, restartCount→1),
		//           run2=ok(errorCount→0, restartCount=1<3, restartCount→2),
		//           run3=err(errorCount=1, restartCount→3),
		//           run4=ok(errorCount→0, restartCount=3>=3 → stop)
		require.Equal(t, 4, callCount)
	})

	t.Run("panic recovery", func(t *testing.T) {
		callCount := 0
		fn := Func(func(ctx context.Context) error {
			callCount++
			panic("boom")
		})

		r := Restart(fn).ErrorLimit(3).
			ErrorBackoff(func(int) time.Duration { return 0 })
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

			r := Restart(fn).ErrorBackoff(func(n int) time.Duration {
				return 10 * time.Second
			})

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
			r := Restart(fn).
				ErrorResetAfter(30 * time.Minute).
				ErrorBackoff(func(n int) time.Duration {
					backoffCalls = append(backoffCalls, n)
					return 0
				})

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

func ExampleRestart() {
	ctx, cancel := initializeForExample()
	defer cancel()

	worker := newDyingRunnable()
	r := Restart(worker).ErrorLimit(3)
	_ = r.Run(ctx)

	// Output:
	// level=INFO msg=starting runnable=restart/dyingRunnable restart=0 errors=0
	// level=INFO msg=starting runnable=restart/dyingRunnable restart=1 errors=1
	// level=INFO msg=starting runnable=restart/dyingRunnable restart=2 errors=2
	// level=INFO msg="not restarting" runnable=restart/dyingRunnable reason="error limit" limit=3
}

func ExampleRestart_worker() {
	ctx, cancel := initializeForExample()
	defer cancel()

	worker := newCounterRunnable()
	r := Restart(worker).Limit(2).Delay(time.Millisecond)
	_ = r.Run(ctx)

	// Output:
	// level=INFO msg=starting runnable=restart/counter restart=0 errors=0
	// level=INFO msg=starting runnable=restart/counter restart=1 errors=0
	// level=INFO msg=starting runnable=restart/counter restart=2 errors=0
	// level=INFO msg="not restarting" runnable=restart/counter reason="restart limit" limit=2
}
