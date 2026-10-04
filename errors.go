package runnable

import "errors"

// ErrShutdownTimeout is reported by [Manager] for each runnable still running when
// the timeout of its shutdown phase expires.
var ErrShutdownTimeout = errors.New("still running after shutdown timeout")
