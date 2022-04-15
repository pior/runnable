package runnable

import (
	"context"
)

// ContextValues returns a new context.Context with the values from the parent, without propagating the cancellation.
// Useful when you want to protect an operation that should not be cancelled.
// Often used with context.WithTimeout() or context.WithDeadline().
func ContextValues(parent context.Context) context.Context {
	return valuesCtx{context.Background(), parent}
}

type valuesCtx struct {
	context.Context
	parent context.Context
}

func (c valuesCtx) Value(key interface{}) interface{} {
	return c.parent.Value(key)
}
