package runnable

import (
	"context"
	stdlog "log"
	"os"
)

func ExampleComponent_cancelled() {
	ctx, cancel := initializeForExample()
	defer cancel()

	c := Component()
	c.AddProcess(newDummyRunnable())

	_ = c.Run(ctx)

	// Output:
	// component: starting services
	// component: starting processes
	// dummyRunnable: started
	// component: context cancelled
	// component: shutting down processes
	// dummyRunnable: stopped
	// component: shutting down services
	// component: shutdown complete
}

func ExampleComponent_failing() {
	ctx := context.Background()

	SetLogger(stdlog.New(os.Stdout, "", 0))

	c := Component()
	c.AddProcess(newDyingRunnable())

	_ = c.Run(ctx)

	// Output:
	// component: starting services
	// component: starting processes
	// dyingRunnable: started
	// dyingRunnable: stopped with error: dying
	// component: a process stopped
	// component: shutting down processes
	// component: shutting down services
	// component: shutdown complete
}
