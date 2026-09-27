package runnable

import (
	"context"
	"os"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

func TestSignal(t *testing.T) {
	// sendSignal returns a runnable that sends sig to its own process, then waits
	// for the cancellation. Signal listens before running it.
	sendSignal := func(sig os.Signal) Runnable {
		return Named("app", Func(func(ctx context.Context) error {
			p, err := os.FindProcess(os.Getpid())
			if err != nil {
				return err
			}
			if sigErr := p.Signal(sig); sigErr != nil {
				return sigErr
			}
			<-ctx.Done()
			return ctx.Err()
		}))
	}

	t.Run("cancels the context on signal", func(t *testing.T) {
		logs := captureLogs(t)

		err := Signal(sendSignal(syscall.SIGHUP), syscall.SIGHUP).Run(context.Background())
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, `level=INFO msg="signal/app: received signal" signal=hangup`+"\n", logs.String())
	})

	t.Run("stops listening when Run returns", func(t *testing.T) {
		// synctest fails with a deadlock if a goroutine is left blocked.
		synctest.Test(t, func(t *testing.T) {
			err := Signal(Func(func(context.Context) error { return nil })).Run(context.Background())
			require.NoError(t, err)
		})
	})

	t.Run("stops listening when cancelled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			err := Signal(Noop()).Run(cancelledContext())
			require.ErrorIs(t, err, context.Canceled)
		})
	})
}
