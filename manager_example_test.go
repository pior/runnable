package runnable_test

import (
	"context"
	"fmt"

	"github.com/pior/runnable"
)

// App holds its manager in a field, which requires the exported Manager type.
type App struct {
	manager *runnable.Manager
}

// buildManager returns a configured manager, which requires the exported Manager type.
func buildManager() *runnable.Manager {
	m := runnable.NewManager()
	registerJobs(m)
	return m
}

// registerJobs only needs to register runnables, not to run the manager.
func registerJobs(r runnable.ManagerRegistry) {
	r.RegisterService(&JobQueue{})
}

func ExampleManagerRegistry() {
	runnable.SetLogger(exampleLogger())

	app := &App{manager: buildManager()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fmt.Println(app.manager.Run(ctx))

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}
