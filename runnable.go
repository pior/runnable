package runnable

import (
	"context"
	"fmt"
	"strings"
)

// Runnable is the contract for anything that runs with a Go context, respects the concellation contract,
// and expects the caller to handle errors.
type Runnable interface {
	Run(context.Context) error
}

type RunnableInit interface {
	Runnable
	Init(context.Context) error
}

type RunnableCleanup interface {
	Runnable
	Cleanup(context.Context) error
}

type RunnableInitCleanup interface {
	Runnable
	Init(context.Context) error
	Cleanup(context.Context) error
}

func nameOfRunnable(runnable Runnable) string {
	if r, ok := runnable.(interface{ name() string }); ok {
		return r.name()
	}
	return strings.TrimLeft(fmt.Sprintf("%T", runnable), "*")
}
