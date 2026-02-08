package runnable

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_runnableName(t *testing.T) {
	require.Equal(t, "github.com/pior/runnable.Test_runnableName.func1",
		runnableName(Func(func(ctx context.Context) error { return nil })),
	)

	require.Equal(t, "github.com/pior/runnable.funcTesting",
		runnableName(Func(funcTesting)),
	)

	require.Equal(t, "custom-name",
		runnableName(FuncNamed("custom-name", funcTesting)),
	)

	require.Equal(t, "dummyRunnable",
		runnableName(newDummyRunnable()),
	)

	require.Equal(t, "restart/dummyRunnable",
		runnableName(Restart(newDummyRunnable())),
	)

	require.Equal(t, "schedule/dummyRunnable",
		runnableName(Schedule(newDummyRunnable(), Every(0))),
	)

	require.Equal(t, "recover/dummyRunnable",
		runnableName(Recover(newDummyRunnable())),
	)

	require.Equal(t, "restart/closer/dummyCloser",
		runnableName(Restart(CloserErr(&dummyCloser{}))),
	)

	require.Equal(t, "restart/recover/closer/dummyCloser",
		runnableName(Restart(Recover(CloserErr(&dummyCloser{})))),
	)
}
