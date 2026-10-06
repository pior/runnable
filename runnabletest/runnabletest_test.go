package runnabletest_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pior/runnable"
	"github.com/pior/runnable/runnabletest"
)

// failingCases are run in a child test process by TestAssertCancellationFails,
// since a failing assertion fails the test that runs it.
var failingCases = map[string]runnable.Runnable{
	"ignores cancellation": runnable.Func(func(context.Context) error {
		time.Sleep(time.Second)
		return nil
	}),
	"returns an error": runnable.Func(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("boom")
	}),
	"returns deadline exceeded": runnable.Func(func(ctx context.Context) error {
		<-ctx.Done()
		return context.DeadlineExceeded
	}),
}

func TestAssertCancellation(t *testing.T) {
	t.Run("returns the context error", func(t *testing.T) {
		runnabletest.AssertCancellation(t, runnable.Func(func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}), time.Second)
	})

	t.Run("returns nil", func(t *testing.T) {
		runnabletest.AssertCancellation(t, runnable.Func(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}), time.Second)
	})
}

func TestAssertCancellationFails(t *testing.T) {
	if name := os.Getenv("RUNNABLETEST_CASE"); name != "" {
		runnabletest.AssertCancellation(t, failingCases[name], 50*time.Millisecond)
		return
	}

	for name := range failingCases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestAssertCancellationFails$")
			cmd.Env = append(os.Environ(), "RUNNABLETEST_CASE="+name)
			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "child test passed:\n%s", out)
			require.Contains(t, string(out), "cancellation")
			// The failure points at the caller, not inside runnabletest.
			require.Contains(t, string(out), " runnabletest_test.go:")
			require.NotContains(t, string(out), " runnabletest.go:")
		})
	}
}
