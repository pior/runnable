package runnable

import (
	"context"
	"reflect"
	"runtime"
	"strings"
)

// Runnable is the contract for anything that runs with a Go context, respects the concellation contract,
// and expects the caller to handle errors.
type Runnable interface {
	Run(context.Context) error
}

func findName(t interface{}) string {
	var parts []string

	for t != nil {
		part := findNameFromOne(t)
		if part != "" {
			parts = append(parts, part)
		}

		if r, ok := t.(interface{ RunnableUnwrap() any }); ok {
			t = r.RunnableUnwrap()
			continue
		}

		break
	}

	return strings.Join(parts, "/")
}

func findNameFromOne(t any) string {
	if r, ok := t.(interface{ RunnableName() string }); ok {
		return r.RunnableName()
	}

	valueOf := reflect.ValueOf(t)
	if valueOf.Kind() == reflect.Func {
		return runtime.FuncForPC(valueOf.Pointer()).Name() + "()"
	}
	return reflect.Indirect(valueOf).Type().Name()
}

type baseWrapper struct {
	name    string
	wrapped any
}

func (w *baseWrapper) RunnableName() string {
	return w.name
}

func (w *baseWrapper) RunnableUnwrap() any {
	return w.wrapped
}
