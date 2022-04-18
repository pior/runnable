package runnable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContextValues(t *testing.T) {
	parent := context.Background()
	parent = context.WithValue(parent, "key", "value")
	parent, cancel := context.WithCancel(parent)
	cancel()

	require.Error(t, parent.Err())

	ctx := ContextValues(parent)
	require.Equal(t, "value", ctx.Value("key"))
	require.NoError(t, ctx.Err())
}
