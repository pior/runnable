package runnable

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func panickingFunc(value any) Runnable {
	return Func(func(context.Context) error {
		panic(value)
	})
}

func runRecover(t *testing.T, value any) *PanicError {
	t.Helper()
	err := Recover(panickingFunc(value)).Run(context.Background())

	var panicErr *PanicError
	require.ErrorAs(t, err, &panicErr)
	return panicErr
}

func TestRecover(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		err := Recover(Func(func(context.Context) error { return nil })).Run(context.Background())
		require.NoError(t, err)
	})

	t.Run("captures value and stack", func(t *testing.T) {
		panicErr := runRecover(t, "boom")

		require.Equal(t, "boom", fmt.Sprint(panicErr.Value))
		require.Equal(t, "runnable panicked: boom", panicErr.Error())
		require.NotEmpty(t, panicErr.Stack)
		require.Contains(t, string(panicErr.Stack), "panickingFunc")
	})

	t.Run("format", func(t *testing.T) {
		panicErr := runRecover(t, "boom")

		require.Equal(t, "runnable panicked: boom", fmt.Sprintf("%v", panicErr))
		require.Equal(t, "runnable panicked: boom", fmt.Sprintf("%s", panicErr))
		require.Equal(t, `"runnable panicked: boom"`, fmt.Sprintf("%q", panicErr))

		detailed := fmt.Sprintf("%+v", panicErr)
		require.Contains(t, detailed, "runnable panicked: boom\ngoroutine ")
		require.Contains(t, detailed, "panickingFunc")
		require.NotContains(t, fmt.Sprintf("%v", panicErr), "goroutine")
	})

	t.Run("unwrap error value", func(t *testing.T) {
		cause := errors.New("cause")
		panicErr := runRecover(t, cause)

		require.ErrorIs(t, panicErr, cause)
		require.Equal(t, "cause", errors.Unwrap(panicErr).Error())
	})

	t.Run("unwrap non-error value", func(t *testing.T) {
		panicErr := runRecover(t, 42)

		require.NoError(t, errors.Unwrap(panicErr))
	})
}
