package runnable

import (
	"context"
)

// ContextValues returns a new context.Context with the values from the parent, without propagating the cancellation.
// Useful when you want to control when the downstream code is cancelled.
func ContextValues(parent context.Context) context.Context {
	return valueContext{context.Background(), parent}
}

type valueContext struct {
	context.Context
	parent context.Context
}

func (c valueContext) Value(key interface{}) interface{} {
	return c.parent.Value(key)
}
