package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pior/runnable"
)

func ExampleRestart() {
	runnable.SetLogger(exampleLogger())

	ctx, cancel := context.WithCancel(context.Background())

	// By default, Restart restarts forever, after success and after errors.
	runs := 0
	worker := runnable.Named(runnable.Func(func(context.Context) error {
		runs++
		if runs == 3 {
			cancel()
		}
		if runs == 2 {
			return errors.New("failed")
		}
		return nil
	}), "worker")

	fmt.Println(runnable.Restart(worker).Run(ctx))

	// Output:
	// level=INFO msg="restart/worker: starting" restart=0 errors=0
	// level=INFO msg="restart/worker: starting" restart=1 errors=0
	// level=INFO msg="restart/worker: starting" restart=2 errors=1
	// context canceled
}

func ExampleRestart_options() {
	runnable.SetLogger(exampleLogger())

	runs := 0
	worker := runnable.Named(runnable.Func(func(context.Context) error {
		runs++
		if runs <= 2 {
			return errors.New("failed")
		}
		return nil
	}), "worker")

	r := runnable.Restart(worker,
		// Stop after 2 restarts following a successful run.
		runnable.RestartLimit(2),
		runnable.RestartDelay(10*time.Millisecond),
		// Give up after 5 consecutive errors, returning the last one.
		runnable.RestartErrorLimit(5),
		runnable.ErrorBackoff(func(errors int) time.Duration {
			return time.Duration(errors) * 10 * time.Millisecond
		}),
		// A run lasting a minute before failing resets the error count.
		runnable.ErrorResetAfter(time.Minute),
	)

	fmt.Println(r.Run(context.Background()))

	// Output:
	// level=INFO msg="restart/worker: starting" restart=0 errors=0
	// level=INFO msg="restart/worker: starting" restart=1 errors=1
	// level=INFO msg="restart/worker: starting" restart=2 errors=2
	// level=INFO msg="restart/worker: not restarting" reason="restart limit" limit=2
	// <nil>
}

func ExampleRestart_errorLimit() {
	runnable.SetLogger(exampleLogger())

	connect := runnable.Named(runnable.Func(func(context.Context) error {
		return errors.New("connection refused")
	}), "connect")

	fmt.Println(runnable.Restart(connect, runnable.RestartErrorLimit(3)).Run(context.Background()))

	// Output:
	// level=INFO msg="restart/connect: starting" restart=0 errors=0
	// level=INFO msg="restart/connect: starting" restart=1 errors=1
	// level=INFO msg="restart/connect: starting" restart=2 errors=2
	// level=INFO msg="restart/connect: not restarting" reason="error limit" limit=3
	// connection refused
}
