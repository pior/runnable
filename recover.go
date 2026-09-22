package runnable

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
)

// PanicError is returned by [Recover] when the wrapped runnable panics.
type PanicError struct {
	// Value is the value passed to panic.
	Value any
	// Stack is the goroutine stack trace captured when the panic was recovered.
	Stack []byte
}

var _ fmt.Formatter = (*PanicError)(nil)

func (e *PanicError) Error() string {
	return fmt.Sprintf("runnable panicked: %s", e.Value)
}

func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// Format implements [fmt.Formatter]: %+v prints the error followed by the stack trace,
// %v and %s print the error, %q prints the error quoted.
func (e *PanicError) Format(f fmt.State, verb rune) {
	switch verb {
	case 'v':
		_, _ = io.WriteString(f, e.Error())
		if f.Flag('+') {
			_, _ = io.WriteString(f, "\n")
			_, _ = f.Write(e.Stack)
		}
	case 's':
		_, _ = io.WriteString(f, e.Error())
	case 'q':
		_, _ = io.WriteString(f, strconv.Quote(e.Error()))
	default:
		_, _ = fmt.Fprintf(f, "%%!%c(*runnable.PanicError=%s)", verb, e.Error())
	}
}

// Recover returns a runnable that recovers when a runnable panics and return an error to represent this panic.
func Recover(runnable Runnable) Runnable {
	return &recoverRunner{"recover/" + runnableName(runnable), runnable}
}

type recoverRunner struct {
	name     string
	runnable Runnable
}

func (r *recoverRunner) runnableName() string { return r.name }

func (r *recoverRunner) Run(ctx context.Context) (err error) {
	defer func() {
		if value := recover(); value != nil {
			err = &PanicError{Value: value, Stack: debug.Stack()}
		}
	}()

	return r.runnable.Run(ctx)
}
