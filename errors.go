package runnable

import "fmt"

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
