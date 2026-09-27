package runnable

import (
	"context"
	"os"
	ossignal "os/signal"
	"syscall"
)

// Signal returns a runnable that runs the given runnable and cancels its context
// when the process receives one of the signals, [syscall.SIGINT] and
// [syscall.SIGTERM] by default. It stops listening when Run returns, and after
// the first signal so that a second one terminates the process.
func Signal(runnable Runnable, signals ...os.Signal) Runnable {
	if len(signals) == 0 {
		signals = append(signals, syscall.SIGINT)
		signals = append(signals, syscall.SIGTERM)
	}

	return &signal{
		name:     "signal/" + runnableName(runnable),
		runnable: runnable,
		signals:  signals,
	}
}

type signal struct {
	name     string
	runnable Runnable
	signals  []os.Signal
}

func (s *signal) runnableName() string { return s.name }

func (s *signal) Run(ctx context.Context) error {
	name := resolveName(ctx, s.name)

	ctx, cancelFunc := context.WithCancel(ctx)
	defer cancelFunc()

	sigChan := make(chan os.Signal, 1)
	ossignal.Notify(sigChan, s.signals...)
	defer ossignal.Stop(sigChan)

	go func() {
		select {
		case sig := <-sigChan:
			// Stop listening, so a second signal terminates the process.
			ossignal.Stop(sigChan)
			logger.Info(name+": received signal", "signal", sig)
			cancelFunc()
		case <-ctx.Done():
		}
	}()

	return s.runnable.Run(ctx)
}
