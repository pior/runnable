package runnable

import (
	"context"
	"reflect"
	"runtime"
)

// RunnableFunc is a function that implements the Runnable contract.
type RunnableFunc func(context.Context) error

type funcRunnable struct {
	name string
	fn   RunnableFunc
}

func (f *funcRunnable) runnableName() string { return f.name }

// Func returns a [Runnable] from a function. Its name is the function name, as
// reported by [runtime.FuncForPC].
func Func(fn RunnableFunc) Runnable {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return &funcRunnable{name, fn}
}

func (f *funcRunnable) Run(ctx context.Context) error {
	return f.fn(ctx)
}
