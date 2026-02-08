package runnable

import (
	"context"
	"time"
)

// ScheduleSpec computes the next execution time given the last execution start
// and the current time. Interval specs like [Every] use lastStart to account
// for execution time. Clock-aligned specs like [DailyAt] use now.
type ScheduleSpec func(lastStart, now time.Time) time.Time

// Schedule returns a runnable that runs the given runnable according to the
// provided schedule specs. When multiple specs are provided, the runnable
// runs at whichever fires next.
//
// If an execution outlasts the interval, missed ticks are skipped (not queued).
// On error from the inner runnable, Schedule stops and returns the error.
// On context cancellation, returns [context.Canceled].
//
// For custom scheduling logic, pass a [ScheduleSpec] function directly.
// For example, to use github.com/robfig/cron/v3:
//
//	sched, _ := cron.ParseStandard("15 */6 * * *") // every 6h at :15
//	Schedule(worker, func(_, now time.Time) time.Time {
//	    return sched.Next(now)
//	})
func Schedule(runnable Runnable, specs ...ScheduleSpec) *schedule {
	return &schedule{
		name:     "schedule/" + runnableName(runnable),
		runnable: runnable,
		specs:    specs,
	}
}

type schedule struct {
	name     string
	runnable Runnable
	specs    []ScheduleSpec
}

func (s *schedule) runnableName() string { return s.name }

// Name sets the runnable name, used in log messages. Defaults to "schedule/<inner>".
func (s *schedule) Name(name string) *schedule {
	s.name = name
	return s
}

func (s *schedule) Run(ctx context.Context) error {
	lastStart := time.Now()

	for {
		next := s.nextTime(lastStart, time.Now())

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(next)):
			lastStart = time.Now()
			if err := s.runnable.Run(ctx); err != nil {
				return err
			}
		}
	}
}

// nextTime returns the earliest next execution time across all specs.
func (s *schedule) nextTime(lastStart, now time.Time) time.Time {
	earliest := s.specs[0](lastStart, now)
	for _, spec := range s.specs[1:] {
		if t := spec(lastStart, now); t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
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
