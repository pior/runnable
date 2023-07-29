package runnable

import "context"

type RunnableFunc func(context.Context) error

type funcRunnable struct {
	baseWrapper
	fn RunnableFunc
}

func Func(fn RunnableFunc) Runnable {
	return &funcRunnable{baseWrapper{"", fn}, fn}
}

func (f *funcRunnable) Run(ctx context.Context) error {
	return f.fn(ctx)
}
