package runnable

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDelay(t *testing.T) {
	t.Run("cancellation", func(t *testing.T) {
		r := Delay(10*time.Millisecond, newDummyRunnable())
		AssertRunnableRespectCancellation(t, r, time.Millisecond*100)
		AssertRunnableRespectPreCancelledContext(t, r)
	})

	t.Run("runs after the delay", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)
			var startedAfter time.Duration
			start := time.Now()
			r := Func(func(context.Context) error {
				startedAfter = time.Since(start)
				return nil
			})

			require.NoError(t, Named("job", Delay(time.Minute, r)).Run(context.Background()))
			require.Equal(t, "1m0s", startedAfter.String())
			require.Equal(t, `level=INFO msg="job: waiting" delay=1m0s`+"\n"+
				`level=INFO msg="job: starting"`+"\n", logs.String())
		})
	})

	t.Run("returns the result of the runnable", func(t *testing.T) {
		err := Delay(0, newDyingRunnable()).Run(context.Background())
		require.EqualError(t, err, "dying")
	})

	t.Run("cancelled during the wait", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			counter := newCounterRunnable()
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(time.Second, cancel)

			err := Delay(time.Minute, counter).Run(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, 0, counter.counter)
		})
	})

	t.Run("name", func(t *testing.T) {
		require.Equal(t, "delay/dummyRunnable", runnableName(Delay(time.Second, newDummyRunnable())))
	})
}
