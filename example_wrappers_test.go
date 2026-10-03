package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/pior/runnable"
)

// Metrics is a service that exports metrics.
type Metrics struct{}

func (*Metrics) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

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
	// A function is named after its Go symbol, Named gives it a readable name
	// for log lines and errors.
	api := runnable.Named("api", runnable.Func(func(context.Context) error {
		return errors.New("port in use")
	}))

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
	var name string
	job := runnable.Func(func(ctx context.Context) error {
		name = runnable.NameFromContext(ctx)
		return nil
	})

	m := runnable.NewManager()
	m.RegisterProcess(runnable.Named("job", job))

	_ = runnable.Named("app", m).Run(context.Background())
	fmt.Println(name)

	// Output:
	// level=INFO msg="app/job: started"
	// level=INFO msg="app/job: stopped"
	// level=INFO msg="app: starting shutdown" reason="job completed"
	// level=INFO msg="app: shutdown complete"
	// app/job
}

// sendSignal returns a runnable that sends sig to its own process, like a user
// pressing Ctrl-C or Kubernetes stopping a pod, then waits for the cancellation.
func sendSignal(sig os.Signal) runnable.Runnable {
	return runnable.Named("app", runnable.Func(func(ctx context.Context) error {
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(sig)
		<-ctx.Done()
		return ctx.Err()
	}))
}

func ExampleSignal() {
	// Signal cancels the context on SIGINT or SIGTERM. Run already does it.
	r := runnable.Signal(sendSignal(syscall.SIGTERM))

	fmt.Println(r.Run(context.Background()))

	// Output:
	// level=INFO msg="signal/app: received signal" signal=terminated
	// context canceled
}

func ExampleSignal_signals() {
	// Cancel the context on SIGHUP only.
	r := runnable.Signal(sendSignal(syscall.SIGHUP), syscall.SIGHUP)

	fmt.Println(r.Run(context.Background()))

	// Output:
	// level=INFO msg="signal/app: received signal" signal=hangup
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
	if pe, ok := errors.AsType[*runnable.PanicError](err); ok {
		fmt.Println("panic value:", pe.Value)
		fmt.Println("has stack:", len(pe.Stack) > 0)
	}

	// Output:
	// panic value: nil map
	// has stack: true
}

func ExampleTimeout() {
	// A job that is stuck, and stops when cancelled.
	job := runnable.Named("report", runnable.Func(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}))

	err := runnable.Timeout(100*time.Millisecond, job).Run(context.Background())
	fmt.Println(err)

	// Output:
	// level=INFO msg="timeout/report: timed out" timeout=100ms
	// timed out after 100ms: context deadline exceeded
}

func ExampleDelay() {
	warmup := runnable.Named("warmup", runnable.Func(func(context.Context) error {
		fmt.Println("warming up")
		return nil
	}))

	fmt.Println(runnable.Delay(10*time.Millisecond, warmup).Run(context.Background()))

	// Output:
	// level=INFO msg="delay/warmup: waiting" delay=10ms
	// level=INFO msg="delay/warmup: starting"
	// warming up
	// <nil>
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
	// A manager shuts down when any runnable returns. Noop is a process that
	// never does, so a manager of services runs until it is cancelled.
	m := runnable.NewManager()
	m.RegisterService(&Metrics{})
	m.RegisterProcess(runnable.Named("idle", runnable.Noop()))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	fmt.Println(m.Run(ctx))

	// Output:
	// level=INFO msg="manager/Metrics: started"
	// level=INFO msg="manager/idle: started"
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/idle: stopped"
	// level=INFO msg="manager/Metrics: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}
