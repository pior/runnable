package runnable_test

import (
	"context"
	"fmt"

	"github.com/pior/runnable"
)

// Mailer is a service that sends queued emails.
type Mailer struct{}

func (*Mailer) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func ExampleRun() {
	// Run is meant for main: it stops on SIGINT or SIGTERM and exits the process
	// with log.Fatal on error.
	m := runnable.NewManager()
	m.RegisterService(&Mailer{})
	m.RegisterProcess(runnable.Named("app", runnable.Func(func(context.Context) error {
		return nil
	})))

	runnable.Run(m)

	// Output:
	// level=INFO msg="manager/Mailer: started"
	// level=INFO msg="manager/app: started"
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app completed"
	// level=INFO msg="manager/Mailer: stopped"
	// level=INFO msg="manager: shutdown complete"
}

func ExampleRunFunc() {
	runnable.RunFunc(func(context.Context) error {
		fmt.Println("hello")
		return nil
	})

	// Output:
	// hello
}

func ExampleRunGroup() {
	// RunGroup runs the runnables as processes of a manager, under Run.
	runnable.RunGroup(
		runnable.Named("migrate", runnable.Func(func(context.Context) error { return nil })),
	)

	// Output:
	// level=INFO msg="manager/migrate: started"
	// level=INFO msg="manager/migrate: stopped"
	// level=INFO msg="manager: starting shutdown" reason="migrate completed"
	// level=INFO msg="manager: shutdown complete"
}
