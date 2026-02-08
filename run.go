package runnable

import (
	"context"
	"errors"
	stdlog "log"
)

// RunGroup runs all runnables in a Manager, and listens to SIGTERM/SIGINT.
func RunGroup(runners ...Runnable) {
	m := Manager()
	m.Register(runners...)
	Run(m)
}

// Run runs a single runnable, and listens to SIGTERM/SIGINT.
func Run(runner Runnable) {
	ctx := context.Background()
	err := Signal(runner).Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		stdlog.Fatal(err)
	}
}

// RunFunc runs a runnable function, and listens to SIGTERM/SIGINT.
func RunFunc(fn RunnableFunc) {
	Run(Func(fn))
}
