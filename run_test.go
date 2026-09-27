package runnable

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	// Run exits the process on error, so the failing case runs in a child process.
	if os.Getenv("RUNNABLE_TEST_RUN_EXIT") == "1" {
		Run(Named("app", Func(func(context.Context) error { return errors.New("boom") })))
		return
	}

	t.Run("exits with status 1 and logs the error", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRun$")
		cmd.Env = append(os.Environ(), "RUNNABLE_TEST_RUN_EXIT=1")
		out, err := cmd.CombinedOutput()

		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		require.Equal(t, 1, exitErr.ExitCode())
		require.Equal(t, `level=INFO msg="app: stopped with error" error=boom`+"\n", string(out))
	})

	t.Run("returns on success", func(t *testing.T) {
		Run(Func(func(context.Context) error { return nil }))
	})

	t.Run("returns on context.Canceled", func(t *testing.T) {
		Run(Func(func(context.Context) error { return context.Canceled }))
	})
}
