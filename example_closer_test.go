package runnable_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/pior/runnable"
)

type Pool struct{}

func (Pool) Close() { fmt.Println("pool closed") }

type DB struct{ err error }

func (db DB) Close() error {
	fmt.Println("db closed")
	return db.err
}

type Tracer struct{}

func (Tracer) Close(context.Context) { fmt.Println("tracer flushed") }

type Client struct{}

func (Client) Close(context.Context) error {
	fmt.Println("client closed")
	return nil
}

func ExampleCloser() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Close is called when the context is cancelled.
	fmt.Println(runnable.Closer(Pool{}).Run(ctx))

	// Output:
	// pool closed
	// <nil>
}

func ExampleCloser_manager() {
	runnable.SetLogger(exampleLogger())

	// As a service, the pool is closed after all processes stopped.
	m := runnable.NewManager()
	m.RegisterService(runnable.Closer(Pool{}))
	m.RegisterProcess(runnable.Named("app", runnable.Func(func(context.Context) error {
		return nil
	})))

	fmt.Println(m.Run(context.Background()))

	// Output:
	// level=INFO msg="manager/closer/Pool: started"
	// level=INFO msg="manager/app: started"
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app completed"
	// pool closed
	// level=INFO msg="manager/closer/Pool: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}

func ExampleCloserErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runnable.CloserErr(DB{err: errors.New("connection busy")}).Run(ctx)
	fmt.Println(err)

	var re *runnable.RunnableError
	fmt.Println(errors.As(err, &re))

	// Output:
	// db closed
	// closer: Close() returned an error: connection busy
	// true
}

func ExampleCloserCtx() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Close receives a context that is not cancelled, to finish its cleanup.
	fmt.Println(runnable.CloserCtx(Tracer{}).Run(ctx))

	// Output:
	// tracer flushed
	// <nil>
}

func ExampleCloserCtxErr() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	fmt.Println(runnable.CloserCtxErr(Client{}).Run(ctx))

	// Output:
	// client closed
	// <nil>
}
