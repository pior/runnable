package runnable

import (
	"context"
	"errors"
	"os"
)

// RunGroup runs the runnables as processes of a [Manager] under [Run].
func RunGroup(runners ...Runnable) {
	m := NewManager()
	m.RegisterProcess(runners...)
	Run(m)
}

// Run runs a runnable until it returns or the process receives SIGINT or
// SIGTERM. On any error other than [context.Canceled], it logs the error with
// the logger set by [SetLogger] and exits the process with status 1, without
// running deferred functions. It is intended as a main helper.
func Run(runner Runnable) {
	err := Signal(runner).Run(context.Background())
	if err != nil && !errors.Is(err, context.Canceled) {
		logCompleted(runnableName(runner), err)
		os.Exit(1)
	}
}

// RunFunc is [Run] for a function.
func RunFunc(fn RunnableFunc) {
	Run(Func(fn))
}
