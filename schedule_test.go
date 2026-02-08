package runnable

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduleSpec_Every(t *testing.T) {
	spec := Every(10 * time.Second)

	t.Run("normal interval", func(t *testing.T) {
		lastStart := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
		now := time.Date(2025, 1, 1, 12, 0, 5, 0, time.UTC)

		got := spec(lastStart, now)
		want := time.Date(2025, 1, 1, 12, 0, 10, 0, time.UTC)

		require.Equal(t, want.String(), got.String())
	})

	t.Run("overdue clamps to now", func(t *testing.T) {
		lastStart := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
		now := time.Date(2025, 1, 1, 12, 0, 15, 0, time.UTC)

		got := spec(lastStart, now)

		require.Equal(t, now.String(), got.String())
	})
}

func TestScheduleSpec_HourlyAt(t *testing.T) {
	spec := HourlyAt(30)

	t.Run("before target minute", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 14, 20, 0, 0, time.UTC)

		got := spec(time.Time{}, now)
		want := time.Date(2025, 1, 1, 14, 30, 0, 0, time.UTC)

		require.Equal(t, want.String(), got.String())
	})

	t.Run("after target minute", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 14, 35, 0, 0, time.UTC)

		got := spec(time.Time{}, now)
		want := time.Date(2025, 1, 1, 15, 30, 0, 0, time.UTC)

		require.Equal(t, want.String(), got.String())
	})
}

func TestScheduleSpec_DailyAt(t *testing.T) {
	spec := DailyAt(8, 0)

	t.Run("before target time", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 7, 0, 0, 0, time.UTC)

		got := spec(time.Time{}, now)
		want := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)

		require.Equal(t, want.String(), got.String())
	})

	t.Run("after target time", func(t *testing.T) {
		now := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)

		got := spec(time.Time{}, now)
		want := time.Date(2025, 1, 2, 8, 0, 0, 0, time.UTC)

		require.Equal(t, want.String(), got.String())
	})
}

func TestScheduleSpec_Hourly(t *testing.T) {
	spec := Hourly()

	now := time.Date(2025, 1, 1, 14, 20, 0, 0, time.UTC)

	got := spec(time.Time{}, now)
	want := time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC)

	require.Equal(t, want.String(), got.String())
}

func TestScheduleSpec_MultipleSpecs(t *testing.T) {
	s := Schedule(newDummyRunnable(), Every(time.Hour), HourlyAt(30))

	lastStart := time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)
	now := time.Date(2025, 1, 1, 14, 20, 0, 0, time.UTC)

	got := s.nextTime(lastStart, now)
	want := time.Date(2025, 1, 1, 14, 30, 0, 0, time.UTC) // HourlyAt(30) fires first

	require.Equal(t, want.String(), got.String())
}

func TestSchedule_Cancellation(t *testing.T) {
	runner := Schedule(newDummyRunnable(), Every(time.Second))

	AssertRunnableRespectCancellation(t, runner, time.Second)
	AssertRunnableRespectPreCancelledContext(t, runner)
}

func TestSchedule_ExecutionCount(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int64
		worker := Func(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error)

		go func() { errChan <- Schedule(worker, Every(10*time.Second)).Run(ctx) }()

		// First tick at 10s
		time.Sleep(10 * time.Second)
		synctest.Wait()
		require.Equal(t, "1", itoa(count.Load()))

		// Second tick at 20s
		time.Sleep(10 * time.Second)
		synctest.Wait()
		require.Equal(t, "2", itoa(count.Load()))

		// Third tick at 30s
		time.Sleep(10 * time.Second)
		synctest.Wait()
		require.Equal(t, "3", itoa(count.Load()))

		cancel()
		err := <-errChan
		require.EqualError(t, err, "context canceled")
	})
}

func TestSchedule_SlowRunnable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int64
		worker := Func(func(ctx context.Context) error {
			count.Add(1)
			time.Sleep(8 * time.Second) // takes 8s out of 10s interval
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error)

		go func() { errChan <- Schedule(worker, Every(10*time.Second)).Run(ctx) }()

		// First execution at 10s, finishes at 18s. Next at max(10+10=20, 18)=20s.
		time.Sleep(18 * time.Second)
		synctest.Wait()
		require.Equal(t, "1", itoa(count.Load()))

		// Second execution at 20s, finishes at 28s.
		time.Sleep(10 * time.Second) // now at 28s
		synctest.Wait()
		require.Equal(t, "2", itoa(count.Load()))

		cancel()
		err := <-errChan
		require.EqualError(t, err, "context canceled")
	})
}

func TestSchedule_MultipleSpecs_Integration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var count atomic.Int64
		worker := Func(func(ctx context.Context) error {
			count.Add(1)
			return nil
		})

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error)

		// Every 30min and HourlyAt(15) — should fire at whichever comes first.
		go func() {
			errChan <- Schedule(worker, Every(30*time.Minute), HourlyAt(15)).Run(ctx)
		}()

		// HourlyAt(15) fires 15min into the hour, Every(30min) fires at 30min.
		// HourlyAt(15) should fire first.
		time.Sleep(15 * time.Minute)
		synctest.Wait()
		require.Equal(t, "1", itoa(count.Load()))

		cancel()
		err := <-errChan
		require.EqualError(t, err, "context canceled")
	})
}

func TestSchedule_ErrorStopsLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		worker := Func(func(ctx context.Context) error {
			return &dummyError{message: "task failed"}
		})

		errChan := make(chan error)
		go func() {
			errChan <- Schedule(worker, Every(time.Second)).Run(context.Background())
		}()

		time.Sleep(time.Second)
		synctest.Wait()

		err := <-errChan
		require.EqualError(t, err, "task failed")
	})
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
