package runnable

import (
	"context"
	"os"
	ossignal "os/signal"
	"syscall"
)

// Signal returns a runnable that runs the runnable and cancels it when the process receives a POSIX signal.
func Signal(runnable Runnable, signals ...os.Signal) Runnable {
	if len(signals) == 0 {
		signals = append(signals, syscall.SIGINT)
		signals = append(signals, syscall.SIGTERM)
	}

	return &signal{runnable, signals}
}

type signal struct {
	runnable Runnable
	signals  []os.Signal
}

func (s *signal) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)

	sigChan := make(chan os.Signal, 1)
	ossignal.Notify(sigChan, s.signals...)

	go func() {
		defer ossignal.Reset(s.signals...)

		sig := <-sigChan
		Log(s, "received signal %s", sig)
		cancelFunc()
	}()

	return s.runnable.Run(ctx)
}
