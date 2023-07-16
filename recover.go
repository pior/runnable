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

// Recover returns a runnable that recovers when a runnable panics and return an error to represent this panic.
func Recover(runnable Runnable) Runnable {
	return &RecoverRunner{baseWrapper{"recover", runnable}, runnable}
}

type RecoverRunner struct {
	baseWrapper
	runnable Runnable
}

func (r *RecoverRunner) Run(ctx context.Context) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = &PanicError{value}
		}
	}()

	return r.runnable.Run(ctx)
}
