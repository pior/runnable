package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pior/runnable"
)

func ExampleSchedule() {
	ctx, cancel := context.WithCancel(context.Background())

	runs := 0
	cleanup := runnable.Func(func(context.Context) error {
		runs++
		fmt.Println("cleanup", runs)
		if runs == 3 {
			cancel()
		}
		return nil
	})

	fmt.Println(runnable.Schedule(cleanup, runnable.Every(10*time.Millisecond)).Run(ctx))

	// Output:
	// cleanup 1
	// cleanup 2
	// cleanup 3
	// context canceled
}

func ExampleScheduleContinueOnError() {
	ctx, cancel := context.WithCancel(context.Background())

	runs := 0
	report := runnable.Named("report", runnable.Func(func(context.Context) error {
		runs++
		if runs == 1 {
			return errors.New("database unavailable")
		}
		fmt.Println("report", runs)
		cancel()
		return nil
	}))

	// The first run fails: the error is logged, and the report runs again at the next tick.
	r := runnable.Schedule(report, runnable.Every(10*time.Millisecond), runnable.ScheduleContinueOnError())

	fmt.Println(r.Run(ctx))

	// Output:
	// level=INFO msg="schedule/report: failed, continuing" error="database unavailable"
	// report 2
	// context canceled
}

// now is a fixed time for the schedule spec examples: Friday 2026-01-02 10:15.
var now = time.Date(2026, 1, 2, 10, 15, 0, 0, time.UTC)

func ExampleEvery() {
	spec := runnable.Every(time.Hour)

	lastStart := now.Add(-10 * time.Minute)
	fmt.Println(spec(lastStart, now))

	// A run that outlasted the interval is followed immediately.
	lastStart = now.Add(-2 * time.Hour)
	fmt.Println(spec(lastStart, now))

	// Output:
	// 2026-01-02 11:05:00 +0000 UTC
	// 2026-01-02 10:15:00 +0000 UTC
}

func ExampleHourly() {
	fmt.Println(runnable.Hourly()(time.Time{}, now))

	// Output:
	// 2026-01-02 11:00:00 +0000 UTC
}

func ExampleHourlyAt() {
	fmt.Println(runnable.HourlyAt(30)(time.Time{}, now))
	fmt.Println(runnable.HourlyAt(5)(time.Time{}, now))

	// Output:
	// 2026-01-02 10:30:00 +0000 UTC
	// 2026-01-02 11:05:00 +0000 UTC
}

func ExampleDailyAt() {
	fmt.Println(runnable.DailyAt(18, 0)(time.Time{}, now))
	fmt.Println(runnable.DailyAt(3, 0)(time.Time{}, now))

	// Output:
	// 2026-01-02 18:00:00 +0000 UTC
	// 2026-01-03 03:00:00 +0000 UTC
}

// weekdays fires at 09:00 on weekdays. Any type with a Next method works with
// Cron, such as the schedules of github.com/robfig/cron/v3.
type weekdays struct{}

func (weekdays) Next(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
	for !next.After(now) || next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func ExampleCron() {
	spec := runnable.Cron(weekdays{})

	// Friday 10:15, the next weekday 09:00 is Monday.
	fmt.Println(spec(time.Time{}, now))

	// Output:
	// 2026-01-05 09:00:00 +0000 UTC
}
