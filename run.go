package runnable

import (
	"context"
	stdlog "log"
)

// RunGroup runs all runnables in a Group, and listen to SIGTERM/SIGINT
func RunGroup(runners ...Runnable) {
	ctx := context.Background()
	err := Signal(Group(runners...)).Run(ctx)
	if err != nil && err != context.Canceled {
		stdlog.Fatal(err)
	}
}

// Run runs a single runnable, and listen to SIGTERM/SIGINT
func Run(runner Runnable) {
	ctx := context.Background()
	err := Signal(runner).Run(ctx)
	if err != nil && err != context.Canceled {
		stdlog.Fatal(err)
	}
}
