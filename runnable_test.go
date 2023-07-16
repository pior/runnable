package runnable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_findName(t *testing.T) {
	require.Equal(t, "github.com/pior/runnable.Test_findName.func1()",
		findName(Func(func(ctx context.Context) error { return nil })),
	)

	require.Equal(t, "github.com/pior/runnable.funcTesting()",
		findName(Func(funcTesting)),
	)

	require.Equal(t, "dummyRunnable",
		findName(newDummyRunnable()),
	)

	require.Equal(t, "restart/dummyRunnable",
		findName(Restart(newDummyRunnable())),
	)

	require.Equal(t, "every-0s/dummyRunnable",
		findName(Every(newDummyRunnable(), 0)),
	)

	require.Equal(t, "recover/dummyRunnable",
		findName(Recover(newDummyRunnable())),
	)

	require.Equal(t, "restart/recover/closer/dummyCloser",
		findName(Restart(Recover(CloserErr(&dummyCloser{})))),
	)
}
