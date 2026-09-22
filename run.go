package runnable

import (
	"context"
	"errors"
	stdlog "log"
)

// RunGroup runs the runnables as processes of a [Manager] under [Run].
func RunGroup(runners ...Runnable) {
	m := NewManager()
	m.RegisterProcess(runners...)
	Run(m)
}

// Run runs a runnable until it returns or the process receives SIGINT or
// SIGTERM, then calls [log.Fatal] on any error other than [context.Canceled].
// It is intended as a main helper.
func Run(runner Runnable) {
	ctx := context.Background()
	err := Signal(runner).Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		stdlog.Fatal(err)
	}
}

// RunFunc is [Run] for a function.
func RunFunc(fn RunnableFunc) {
	Run(Func(fn))
}
