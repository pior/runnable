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

// Func returns a Runnable from a function. The name is derived from the function using reflection.
func Func(fn RunnableFunc) *funcRunnable {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return &funcRunnable{name, fn}
}

// Name sets the runnable name, used in log messages.
// Defaults to the function name derived via reflection.
func (f *funcRunnable) Name(name string) *funcRunnable {
	f.name = name
	return f
}

func (f *funcRunnable) Run(ctx context.Context) error {
	return f.fn(ctx)
}
