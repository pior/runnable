package runnable

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Signal returns a runnable that runs the runnable and cancels it when the process receives a POSIX signal.
func Signal(runnable Runnable, signals ...os.Signal) Runnable {
	if len(signals) == 0 {
		signals = append(signals, syscall.SIGINT)
		signals = append(signals, syscall.SIGTERM)
	}

	return &signalRunnable{runnable, signals}
}

type signalRunnable struct {
	runnable Runnable
	signals  []os.Signal
}

func (s *signalRunnable) Run(ctx context.Context) error {
	ctx, cancelFunc := context.WithCancel(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, s.signals...)

	go func() {
		defer signal.Reset(s.signals...)

		<-sigChan
		cancelFunc()
	}()

	return s.runnable.Run(ctx)
}
