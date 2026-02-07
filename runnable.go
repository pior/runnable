package runnable

import (
	"context"
	"reflect"
	"runtime"
)

// Runnable is the contract for anything that runs with a Go context, respects the cancellation contract,
// and expects the caller to handle errors.
type Runnable interface {
	Run(context.Context) error
}

// namer is implemented by wrappers to provide a name for logging.
type namer interface {
	runnableName() string
}

// runnableName returns the name of a runnable for logging.
// It checks for the namer interface first, then falls back to reflection.
func runnableName(v any) string {
	if n, ok := v.(namer); ok {
		return n.runnableName()
	}
	valueOf := reflect.ValueOf(v)
	if valueOf.Kind() == reflect.Func {
		return runtime.FuncForPC(valueOf.Pointer()).Name()
	}
	return reflect.Indirect(valueOf).Type().Name()
}
