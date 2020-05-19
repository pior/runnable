package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Periodic_Cancellation(t *testing.T) {
	runner := Periodic(PeriodicOptions{Period: time.Second}, newDummyRunnable())

	AssertRunnableRespectCancellation(t, runner, time.Second)
	AssertRunnableRespectPreCancelledContext(t, runner)
}

func Test_Periodic(t *testing.T) {
	counterRunnable := newCounterRunnable()

	runner := Periodic(PeriodicOptions{Period: time.Millisecond * 10}, counterRunnable)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	err := runner.Run(ctx)
	require.EqualError(t, err, "context deadline exceeded")

	require.GreaterOrEqual(t, counterRunnable.counter, 5)
	require.LessOrEqual(t, counterRunnable.counter, 10)
}

func Test_Periodic_Name(t *testing.T) {
	AssertName(t, "periodic(runnable.dummyRunnable)", Periodic(PeriodicOptions{Period: time.Second}, newDummyRunnable()))
}
