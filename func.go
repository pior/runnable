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
func Func(fn RunnableFunc) Runnable {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	return &funcRunnable{name, fn}
}

// FuncNamed returns a named Runnable from a function.
func FuncNamed(name string, fn RunnableFunc) Runnable {
	return &funcRunnable{name, fn}
}

func (f *funcRunnable) Run(ctx context.Context) error {
	return f.fn(ctx)
}
