package runnable_test

import (
	"context"
	"fmt"

	"github.com/pior/runnable"
)

func ExampleRun() {
	runnable.SetLogger(exampleLogger())

	// Run is meant for main: it stops on SIGINT or SIGTERM and exits the process
	// with log.Fatal on error.
	m := runnable.NewManager()
	m.RegisterService(&JobQueue{})
	m.RegisterProcess(runnable.Named(runnable.Func(func(context.Context) error {
		return nil
	}), "app"))

	runnable.Run(m)

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/app: started"
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app completed"
	// level=INFO msg="manager/JobQueue: stopped"
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
	runnable.SetLogger(exampleLogger())

	// RunGroup runs the runnables as processes of a manager, under Run.
	runnable.RunGroup(
		runnable.Named(runnable.Func(func(context.Context) error { return nil }), "migrate"),
	)

	// Output:
	// level=INFO msg="manager/migrate: started"
	// level=INFO msg="manager/migrate: stopped"
	// level=INFO msg="manager: starting shutdown" reason="migrate completed"
	// level=INFO msg="manager: shutdown complete"
}
