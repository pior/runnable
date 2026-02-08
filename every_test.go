package runnable

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Every_Cancellation(t *testing.T) {
	runner := Every(newDummyRunnable(), time.Second)

	AssertRunnableRespectCancellation(t, runner, time.Second)
	AssertRunnableRespectPreCancelledContext(t, runner)
}

func Test_Every(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int32
		counter := Func(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})
		runner := Every(counter, 10*time.Second)

		ctx, cancel := context.WithCancel(context.Background())

		errChan := make(chan error, 1)
		go func() {
			errChan <- runner.Run(ctx)
		}()

		// Advance time through 3 ticks.
		time.Sleep(30*time.Second + 1)
		require.Equal(t, int32(3), count.Load())

		cancel()
		err := <-errChan
		require.EqualError(t, err, "context canceled")
	})
}
