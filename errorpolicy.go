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
