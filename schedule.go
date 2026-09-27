package runnable

import (
	"context"
	"time"
)

// ScheduleSpec computes the next execution time given the last execution start
// and the current time. Interval specs like [Every] use lastStart to account
// for execution time. Clock-aligned specs like [DailyAt] use now.
type ScheduleSpec func(lastStart, now time.Time) time.Time

// Schedule returns a runnable that runs the given runnable according to spec.
//
// If an execution outlasts the interval, missed ticks are skipped (not queued).
// On error from the inner runnable, Schedule stops and returns the error,
// unless [ScheduleContinueOnError] is set. On context cancellation, returns
// [context.Canceled].
//
// For example:
//
//	runnable.Schedule(cleanup, runnable.DailyAt(3, 0))
//
// To run at several times, use several schedules, or a [Cron] schedule. For
// custom scheduling logic, pass a [ScheduleSpec] function directly, or adapt
// any type with a Next method using [Cron]. For example, to use
// github.com/robfig/cron/v3:
//
//	sched, _ := cron.ParseStandard("15 */6 * * *") // every 6h at :15
//	runnable.Schedule(worker, runnable.Cron(sched))
func Schedule(runnable Runnable, spec ScheduleSpec, opts ...ScheduleOption) Runnable {
	s := &schedule{
		name:     "schedule/" + runnableName(runnable),
		runnable: runnable,
		spec:     spec,
	}
	for _, opt := range opts {
		opt.applySchedule(s)
	}
	return s
}

// ScheduleOption configures [Schedule].
type ScheduleOption interface{ applySchedule(*schedule) }

type scheduleOptionFunc func(*schedule)

func (f scheduleOptionFunc) applySchedule(s *schedule) { f(s) }

// ScheduleContinueOnError makes [Schedule] log an error returned by the
// runnable and run it again at the next tick, instead of stopping and
// returning the error.
func ScheduleContinueOnError() ScheduleOption {
	return scheduleOptionFunc(func(s *schedule) { s.continueOnError = true })
}

type schedule struct {
	name            string
	runnable        Runnable
	spec            ScheduleSpec
	continueOnError bool
}

var _ Runnable = (*schedule)(nil)

func (s *schedule) runnableName() string { return s.name }

func (s *schedule) Run(ctx context.Context) error {
	name := resolveName(ctx, s.name)
	lastStart := time.Now()

	for {
		next := s.spec(lastStart, time.Now())

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(next)):
			lastStart = time.Now()
			err := s.runnable.Run(ctx)
			if err == nil {
				continue
			}
			if !s.continueOnError || ctx.Err() != nil {
				return err
			}
			// Not returned, so the manager never sees it: log it here.
			logger.Info(name+": failed, continuing", "error", err)
		}
	}
}

// Every returns a schedule spec that triggers at regular intervals, accounting
// for execution time. If the runnable takes longer than the interval, the next
// execution starts immediately (missed ticks are skipped, not queued).
func Every(d time.Duration) ScheduleSpec {
	return func(lastStart, now time.Time) time.Time {
		next := lastStart.Add(d)
		if next.Before(now) {
			return now
		}
		return next
	}
}

// Hourly returns a schedule spec that triggers at the top of every hour (:00).
func Hourly() ScheduleSpec {
	return HourlyAt(0)
}

// HourlyAt returns a schedule spec that triggers at the given minute past each hour.
func HourlyAt(minute int) ScheduleSpec {
	return func(_, now time.Time) time.Time {
		next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(time.Hour)
		}
		return next
	}
}

// DailyAt returns a schedule spec that triggers at the given hour and minute each day.
func DailyAt(hour, minute int) ScheduleSpec {
	return func(_, now time.Time) time.Time {
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		return next
	}
}

// Cron returns a schedule spec that delegates to s.Next(now). It adapts any
// schedule exposing a Next method, such as github.com/robfig/cron/v3's
// cron.Schedule, without depending on it.
func Cron(s interface{ Next(time.Time) time.Time }) ScheduleSpec {
	return func(_, now time.Time) time.Time {
		return s.Next(now)
	}
}
