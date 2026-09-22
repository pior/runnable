package runnable

import (
	"context"
	"reflect"
	"runtime"
)

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

type nameKey struct{}

// contextName is the resolved name of the runnable being run.
type contextName struct {
	name string
	// fromManager is true when a parent Manager assigned the name, in which case
	// that Manager also wraps the runnable's error with its name.
	fromManager bool
}

// withName returns a context carrying name as the resolved name of the runnable being run.
func withName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameKey{}, contextName{name: name})
}

// withManagerName is like withName, for a name assigned by a Manager to its child.
func withManagerName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, nameKey{}, contextName{name: name, fromManager: true})
}

func nameFromContext(ctx context.Context) contextName {
	n, _ := ctx.Value(nameKey{}).(contextName)
	return n
}

// NameFromContext returns the full name assigned to the running runnable by its
// parents, such as "manager/restart/JobQueue". Only [Manager] and [Named] assign
// a name, other wrappers pass it through. Returns an empty string when unset.
func NameFromContext(ctx context.Context) string {
	return nameFromContext(ctx).name
}

// resolveName returns the name from the context, or fallback when unset.
func resolveName(ctx context.Context, fallback string) string {
	if name := NameFromContext(ctx); name != "" {
		return name
	}
	return fallback
}

// Named returns a runnable that runs r under the given name. The name is used by a
// parent [Manager] for its log lines and errors, and when no parent assigns one,
// it is passed to r through the context (see [NameFromContext]).
func Named(r Runnable, name string) Runnable {
	return &named{name: name, runnable: r}
}

type named struct {
	name     string
	runnable Runnable
}

var _ Runnable = (*named)(nil)

func (n *named) runnableName() string { return n.name }

func (n *named) Run(ctx context.Context) error {
	if NameFromContext(ctx) == "" {
		ctx = withName(ctx, n.name)
	}
	return n.runnable.Run(ctx)
}
