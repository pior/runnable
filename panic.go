package runnable

import (
	"context"
	"fmt"
)

type PanicError struct {
	value interface{}
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("runnable panicked: %s", e.value)
}

func (e *PanicError) Unwrap() error {
	if err, ok := e.value.(error); ok {
		return err
	}
	return nil
}

// Recover returns a runnable that will periodically run the runnable passed in argument.
func Recover(runnable Runnable) Runnable {
	return &recoverRunnable{runnable}
}

type recoverRunnable struct {
	runnable Runnable
}

func (r *recoverRunnable) Run(ctx context.Context) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = &PanicError{value}
		}
	}()

	return r.runnable.Run(ctx)
}

func (r *recoverRunnable) name() string {
	return nameOfRunnable(r.runnable)
}
