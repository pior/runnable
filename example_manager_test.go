package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pior/runnable"
)

func ExampleNewManager() {
	runnable.SetLogger(exampleLogger())

	app := runnable.Named("app", runnable.Func(func(context.Context) error {
		return nil // the app completes, which shuts the manager down
	}))

	m := runnable.NewManager()
	m.RegisterService(&JobQueue{})
	m.RegisterProcess(app)

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/app: started"
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app completed"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}

func ExampleNewManager_options() {
	runnable.SetLogger(exampleLogger())

	// stuck ignores cancellation, like a process blocked on a call without a context.
	unblock := make(chan struct{})
	defer close(unblock)
	stuck := runnable.Named("stuck", runnable.Func(func(context.Context) error {
		<-unblock
		return nil
	}))

	m := runnable.NewManager(
		runnable.ProcessShutdownTimeout(100*time.Millisecond),
		runnable.ServiceShutdownTimeout(100*time.Millisecond),
	)
	m.RegisterService(&JobQueue{})
	m.RegisterProcess(stuck)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	fmt.Println(err)
	fmt.Println(errors.Is(err, runnable.ErrShutdownTimeout))

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/stuck: started"
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/stuck: still running"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
	// manager: stuck: still running after shutdown timeout
	// true
}

func ExampleNewManager_failure() {
	runnable.SetLogger(exampleLogger())

	worker := runnable.Named("worker", runnable.Func(func(context.Context) error {
		return errors.New("connection lost")
	}))

	m := runnable.NewManager()
	m.RegisterService(&JobQueue{})
	m.RegisterProcess(worker)

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/worker: started"
	// level=INFO msg="manager/worker: stopped with error" error="connection lost"
	// level=INFO msg="manager: starting shutdown" reason="worker died"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
	// manager: worker: connection lost
}

func ExampleNewManager_nested() {
	runnable.SetLogger(slog.New(slog.DiscardHandler))

	// The inner manager stops its own process before its own service, while the
	// outer manager keeps its service running until the inner manager stopped.
	inner := runnable.NewManager()
	inner.RegisterService(runnable.Named("cache", runnable.Noop()))
	inner.RegisterProcess(runnable.Named("api", runnable.Noop()))

	outer := runnable.NewManager()
	outer.RegisterService(runnable.Named("database", runnable.Noop()))
	outer.RegisterProcess(runnable.Named("web", inner))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fmt.Println(outer.Run(ctx))

	// Output:
	// <nil>
}

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
