package runnable

import (
	"errors"
	"fmt"
)

// ErrShutdownTimeout is reported by [Manager] for each runnable still running when
// the shutdown timeout expires.
var ErrShutdownTimeout = errors.New("still running after shutdown timeout")

type RunnableError struct {
	msg string
	err error
}

func (e *RunnableError) Error() string {
	return fmt.Sprintf("%s: %s", e.msg, e.err)
}

func (e *RunnableError) Unwrap() error {
	return e.err
}
