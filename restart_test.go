package runnable

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRestart_Cancellation(t *testing.T) {
	r := Restart(newDummyRunnable())
	AssertRunnableRespectCancellation(t, r, time.Millisecond*100)
	AssertRunnableRespectPreCancelledContext(t, r)
}

func TestRestart_Restart(t *testing.T) {
	counter := newCounterRunnable()

	r := Restart(counter, RestartLimit(10))
	err := r.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, 11, counter.counter) // 10 restarts is 11 executions
}

func TestRestart_Crash_Restart(t *testing.T) {
	counter := newDyingRunnable()

	r := Restart(counter, RestartCrashLimit(10), RestartCrashDelayFn(func(int) time.Duration { return 0 }))
	err := r.Run(context.Background())
	require.EqualError(t, err, "dying")

	require.Equal(t, 10, counter.counter)
}

func TestRestart_Name(t *testing.T) {
	r := Restart(newDummyRunnable())
	AssertName(t, "restart(runnable.dummyRunnable)", r)
}
