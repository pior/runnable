package runnable

import "time"

// ErrorOption configures how [Restart] and [Retry] handle errors.
type ErrorOption interface {
	RestartOption
	RetryOption
}

// errorPolicy is the error handling shared by [Restart] and [Retry].
type errorPolicy struct {
	limit      int
	backoff    func(errors int) time.Duration
	resetAfter time.Duration
	onError    func(error)
}

func newErrorPolicy() errorPolicy {
	return errorPolicy{backoff: defaultErrorBackoff}
}

// countError returns the consecutive error count after a run that failed after
// running for d, and whether it reached the limit.
func (p errorPolicy) countError(count int, d time.Duration) (int, bool) {
	if p.resetAfter > 0 && d >= p.resetAfter {
		count = 0
	}
	count++
	return count, p.limit > 0 && count >= p.limit
}

// notify calls the [OnError] function, if any.
func (p errorPolicy) notify(err error) {
	if p.onError != nil {
		p.onError(err)
	}
}

type errorOptionFunc func(*errorPolicy)

func (f errorOptionFunc) applyRestart(r *restart) { f(&r.errors) }

func (f errorOptionFunc) applyRetry(r *retry) { f(&r.errors) }

var _ ErrorOption = errorOptionFunc(nil)

// ErrorLimit sets the maximum number of consecutive errors. When reached, the
// last error is returned. Zero means unlimited (the default).
func ErrorLimit(n int) ErrorOption {
	return errorOptionFunc(func(p *errorPolicy) { p.limit = n })
}

// ErrorBackoff sets the function that determines the delay before running again
// after an error. It receives the current consecutive error count (starting at
// 1). The default backs off: immediate for the first 3 errors, 10s up to 10,
// then 1m.
func ErrorBackoff(fn func(errors int) time.Duration) ErrorOption {
	return errorOptionFunc(func(p *errorPolicy) { p.backoff = fn })
}

// ExponentialBackoff returns a backoff for [ErrorBackoff] that doubles the
// delay with each consecutive error, from base up to maxDelay: base, 2·base,
// 4·base, and so on.
//
// For example, with a base of 1s and a max of 1m, the delays for the 1st to
// 8th consecutive errors are 1s, 2s, 4s, 8s, 16s, 32s, 1m and 1m:
//
//	runnable.ErrorBackoff(runnable.ExponentialBackoff(time.Second, time.Minute))
//
// With a base of 100ms and a max of 10s, they are 100ms, 200ms, 400ms, 800ms,
// 1.6s, 3.2s, 6.4s and 10s:
//
//	runnable.ErrorBackoff(runnable.ExponentialBackoff(100*time.Millisecond, 10*time.Second))
func ExponentialBackoff(base, maxDelay time.Duration) func(errors int) time.Duration {
	return func(errors int) time.Duration {
		delay := base
		for range errors - 1 {
			if delay >= maxDelay/2 {
				return maxDelay
			}
			delay *= 2
		}
		return min(delay, maxDelay)
	}
}

// ErrorResetAfter resets the consecutive error count when a single run lasted
// at least the given duration before failing. This prevents a long-running
// runnable that occasionally fails from accumulating stale errors into the
// backoff and the limit. Zero means never reset based on duration (the default).
func ErrorResetAfter(d time.Duration) ErrorOption {
	return errorOptionFunc(func(p *errorPolicy) { p.resetAfter = d })
}

// OnError sets a function called with each error of the runnable, including the
// last one when [ErrorLimit] is reached. A recovered panic is a [*PanicError].
// It is not called when the run stops on context cancellation.
func OnError(fn func(err error)) ErrorOption {
	return errorOptionFunc(func(p *errorPolicy) { p.onError = fn })
}

func defaultErrorBackoff(errorCount int) time.Duration {
	switch {
	case errorCount <= 3:
		return 0
	case errorCount <= 10:
		return 10 * time.Second
	default:
		return time.Minute
	}
}
