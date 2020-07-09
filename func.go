package runnable

import "context"

type RunnableFunc func(context.Context) error

type funcRunnable struct {
	fn RunnableFunc
}

func Func(fn RunnableFunc) Runnable {
	return &funcRunnable{fn}
}

func (f *funcRunnable) Run(ctx context.Context) error {
	return f.fn(ctx)
}

func (f *funcRunnable) name() string {
	return composeName("func", f.fn)
}
