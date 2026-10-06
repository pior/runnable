// Package runnabletest checks that a [runnable.Runnable] follows the
// cancellation contract: on cancellation, it returns nil or the context error.
package runnabletest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pior/runnable"
)

// AssertCancellation checks, in subtests, that r returns nil or
// [context.Canceled] within wait when its context is cancelled while running,
// and when its context is already cancelled.
//
//	func TestWorker(t *testing.T) {
//		runnabletest.AssertCancellation(t, NewWorker(), time.Second)
//	}
func AssertCancellation(t *testing.T, r runnable.Runnable, wait time.Duration) {
	// Helper in the subtests too, so a failure points at the caller.
	t.Helper()

	t.Run("cancelled while running", func(t *testing.T) {
		t.Helper()

		ctx, cancel := context.WithCancel(context.Background())
		errc := run(ctx, r)

		// Give the runnable a moment to start before cancelling.
		time.Sleep(10 * time.Millisecond)
		cancel()

		assertStopped(t, errc, wait)
	})

	t.Run("already cancelled", func(t *testing.T) {
		t.Helper()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assertStopped(t, run(ctx, r), wait)
	})
}

func run(ctx context.Context, r runnable.Runnable) <-chan error {
	// Buffered, so the goroutine does not leak when the runnable returns late.
	errc := make(chan error, 1)
	go func() { errc <- r.Run(ctx) }()
	return errc
}

func assertStopped(t *testing.T, errc <-chan error, wait time.Duration) {
	t.Helper()

	select {
	case err := <-errc:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("returned %q on cancellation, want nil or context.Canceled", err)
		}
	case <-time.After(wait):
		t.Fatalf("did not return within %s of cancellation", wait)
	}
}
