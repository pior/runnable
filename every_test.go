package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Every_Cancellation(t *testing.T) {
	runner := Every(newDummyRunnable(), time.Second)

	AssertRunnableRespectCancellation(t, runner, time.Second)
	AssertRunnableRespectPreCancelledContext(t, runner)
}

func Test_Every(t *testing.T) {
	counterRunnable := newCounterRunnable()

	runner := Every(counterRunnable, time.Millisecond*10)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := runner.Run(ctx)
	require.EqualError(t, err, "context deadline exceeded")

	require.GreaterOrEqual(t, counterRunnable.counter, 5)
	require.LessOrEqual(t, counterRunnable.counter, 10)
}

func Test_Every_Name(t *testing.T) {
	AssertName(t, "every[1s](runnable.dummyRunnable)", Every(newDummyRunnable(), time.Second))
}
