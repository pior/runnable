package runnable

import (
	"context"
	"net/http"
	"testing"
	"testing/synctest"

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
		runnableName(Named(Func(funcTesting), "custom-name")),
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

// nameRecorder returns a runnable that records the name from its context and returns nil.
func nameRecorder(got *string) *funcRunnable {
	return Func(func(ctx context.Context) error {
		*got = NameFromContext(ctx)
		return nil
	})
}

func TestNamed(t *testing.T) {
	t.Run("inside a manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)

			m := NewManager()
			m.RegisterProcess(Named(newDyingRunnable(), "worker"))

			err := m.Run(context.Background())
			require.EqualError(t, err, "manager: worker: dying")
			require.Equal(t, `level=INFO msg="manager/worker: started"
level=INFO msg="manager/worker: stopped with error" error=dying
level=INFO msg="manager: starting shutdown" reason="worker died"
level=INFO msg="manager: shutdown complete"
`, logs.String())
		})
	})

	t.Run("name assigned by a manager is kept", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var got string
			m := NewManager()
			m.RegisterProcess(Named(Named(nameRecorder(&got), "inner"), "outer"))

			require.NoError(t, m.Run(context.Background()))
			require.Equal(t, "manager/outer", got)
		})
	})

	t.Run("at top level", func(t *testing.T) {
		var got string
		Run(Named(nameRecorder(&got), "x"))
		require.Equal(t, "x", got)
	})

	t.Run("names a root manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)

			m := NewManager()
			m.RegisterProcess(newDyingRunnable())

			err := Named(m, "app").Run(context.Background())
			require.EqualError(t, err, "app: dyingRunnable: dying")
			require.Contains(t, logs.String(), `msg="app/dyingRunnable: started"`)
		})
	})

	t.Run("HTTPServer inside a manager", func(t *testing.T) {
		logs := captureLogs(t)

		server := &http.Server{Addr: "127.0.0.1:0", Handler: http.NotFoundHandler()}
		m := NewManager()
		m.RegisterProcess(Named(HTTPServer(server), "api"))

		require.NoError(t, m.Run(cancelledContext()))
		require.Contains(t, logs.String(), `msg="manager/api: listening" addr=127.0.0.1:0`)
		require.Contains(t, logs.String(), `msg="manager/api: stopped"`)
	})
}

func TestNameFromContext(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		require.Empty(t, NameFromContext(context.Background()))
	})

	t.Run("Func in a manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var got string
			fn := nameRecorder(&got)

			m := NewManager()
			m.RegisterProcess(fn)

			require.NoError(t, m.Run(context.Background()))
			require.Equal(t, "manager/"+runnableName(fn), got)
		})
	})

	t.Run("passed through wrappers", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)

			var got string
			m := NewManager()
			m.RegisterProcess(Restart(Named(nameRecorder(&got), "job")).Limit(1))

			require.NoError(t, m.Run(context.Background()))
			require.Equal(t, "manager/restart/job", got)
			require.Contains(t, logs.String(), `msg="manager/restart/job: starting"`)
		})
	})

	t.Run("nested manager", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			logs := captureLogs(t)

			var got string
			inner := NewManager()
			inner.RegisterProcess(Named(nameRecorder(&got), "x"))

			outer := NewManager()
			outer.RegisterProcess(Named(inner, "inner"))

			require.NoError(t, outer.Run(context.Background()))
			require.Equal(t, "manager/inner/x", got)
			require.Contains(t, logs.String(), `msg="manager/inner/x: started"`)
			require.Contains(t, logs.String(), `msg="manager/inner: starting shutdown" reason="x died"`)
		})
	})
}
