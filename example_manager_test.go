package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pior/runnable"
)

// Database is a service that the processes depend on.
type Database struct{}

func (*Database) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func ExampleNewManager() {
	app := runnable.Named("app", runnable.Func(func(context.Context) error {
		return nil // the app completes, which shuts the manager down
	}))

	m := runnable.NewManager()
	m.RegisterService(&Database{})
	m.RegisterProcess(app)

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/Database: started"
	// level=INFO msg="manager/app: started"
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app completed"
	// level=INFO msg="manager/Database: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}

func ExampleNewManager_options() {
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
	m.RegisterService(&Database{})
	m.RegisterProcess(stuck)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := m.Run(ctx)
	fmt.Println(err)
	fmt.Println(errors.Is(err, runnable.ErrShutdownTimeout))

	// Output:
	// level=INFO msg="manager/Database: started"
	// level=INFO msg="manager/stuck: started"
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/stuck: still running"
	// level=INFO msg="manager/Database: stopped"
	// level=INFO msg="manager: shutdown complete"
	// manager: stuck: still running after shutdown timeout
	// true
}

func ExampleNewManager_failure() {
	worker := runnable.Named("worker", runnable.Func(func(context.Context) error {
		return errors.New("connection lost")
	}))

	m := runnable.NewManager()
	m.RegisterService(&Database{})
	m.RegisterProcess(worker)

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/Database: started"
	// level=INFO msg="manager/worker: started"
	// level=INFO msg="manager/worker: stopped with error" error="connection lost"
	// level=INFO msg="manager: starting shutdown" reason="worker died"
	// level=INFO msg="manager/Database: stopped"
	// level=INFO msg="manager: shutdown complete"
	// manager: worker: connection lost
}

func ExampleNewManager_nested() {
	// Both managers log concurrently, keep the logs out of the example output.
	runnable.SetLogger(slog.New(slog.DiscardHandler))
	defer runnable.SetLogger(nil)

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
