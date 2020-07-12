package runnable

import (
	"context"
	stdlog "log"
)

// RunGroup runs all runnables in a Group, and listen to SIGTERM/SIGINT
func RunGroup(runners ...Runnable) {
	m := Manager(nil)
	for _, runner := range runners {
		m.Add(runner)
	}
	Run(m.Build())
}

// Run runs a single runnable, and listen to SIGTERM/SIGINT
func Run(runner Runnable) {
	ctx := context.Background()
	err := Signal(runner).Run(ctx)
	if err != nil && err != context.Canceled {
		stdlog.Fatal(err)
	}
}

// RunFunc runs a runnable function, and listen to SIGTERM/SIGINT
func RunFunc(fn RunnableFunc) {
	Run(Func(fn))
}
