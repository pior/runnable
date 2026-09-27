package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"time"

	"github.com/pior/runnable"
)

func ExampleFunc() {
	hello := runnable.Func(func(context.Context) error {
		fmt.Println("hello")
		return nil
	})

	fmt.Println(hello.Run(context.Background()))

	// Output:
	// hello
	// <nil>
}

func ExampleFunc_worker() {
	jobs := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())

	// A worker loops until cancelled, then returns ctx.Err(): a clean stop.
	worker := runnable.Func(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case job := <-jobs:
				fmt.Println("processed", job)
			}
		}
	})

	go func() {
		jobs <- "a"
		jobs <- "b"
		cancel()
	}()

	fmt.Println(worker.Run(ctx))

	// Output:
	// processed a
	// processed b
	// context canceled
}

func ExampleNamed() {
	runnable.SetLogger(exampleLogger())

	// A function is named after its Go symbol, Named gives it a readable name
	// for log lines and errors.
	api := runnable.Named(runnable.Func(func(context.Context) error {
		return errors.New("port in use")
	}), "api")

	m := runnable.NewManager()
	m.RegisterProcess(api)

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/api: started"
	// level=INFO msg="manager/api: stopped with error" error="port in use"
	// level=INFO msg="manager: starting shutdown" reason="api died"
	// level=INFO msg="manager: shutdown complete"
	// manager: api: port in use
}

func ExampleNameFromContext() {
	runnable.SetLogger(slog.New(slog.DiscardHandler))

	job := runnable.Func(func(ctx context.Context) error {
		fmt.Println(runnable.NameFromContext(ctx))
		return nil
	})

	m := runnable.NewManager()
	m.RegisterProcess(runnable.Named(job, "job"))

	_ = runnable.Named(m, "app").Run(context.Background())

	// Output:
	// app/job
}

func ExampleSignal() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Signal also cancels the context on SIGINT or SIGTERM. Run already does it.
	r := runnable.Signal(runnable.Noop())

	fmt.Println(r.Run(ctx))

	// Output:
	// context canceled
}

func ExampleSignal_signals() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Cancel the context on SIGHUP only.
	r := runnable.Signal(runnable.Noop(), syscall.SIGHUP)

	fmt.Println(r.Run(ctx))

	// Output:
	// context canceled
}

func ExampleRecover() {
	r := runnable.Recover(runnable.Func(func(context.Context) error {
		panic("boom")
	}))

	fmt.Println(r.Run(context.Background()))

	// Output:
	// runnable panicked: boom
}

func ExampleRecover_panicError() {
	r := runnable.Recover(runnable.Func(func(context.Context) error {
		panic(errors.New("nil map"))
	}))

	err := r.Run(context.Background())

	// PanicError carries the panic value and the stack, printed by %+v.
	var pe *runnable.PanicError
	if errors.As(err, &pe) {
		fmt.Println("panic value:", pe.Value)
		fmt.Println("has stack:", len(pe.Stack) > 0)
	}

	// Output:
	// panic value: nil map
	// has stack: true
}

func ExampleNoop() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Noop blocks until the context is cancelled.
	fmt.Println(runnable.Noop().Run(ctx))

	// Output:
	// context canceled
}

func ExampleNoop_servicesOnly() {
	runnable.SetLogger(exampleLogger())

	// A manager shuts down when any runnable returns. Noop is a process that
	// never does, so a manager of services runs until it is cancelled.
	m := runnable.NewManager()
	m.RegisterService(&JobQueue{})
	m.RegisterProcess(runnable.Named(runnable.Noop(), "idle"))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	fmt.Println(m.Run(ctx))

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/idle: started"
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/idle: stopped"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}
