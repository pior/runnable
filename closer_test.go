package runnable

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dummyCloser struct {
	err    error
	called int
}

func (c *dummyCloser) Close() error {
	c.called++
	return c.err
}

type dummyCloserCtx struct {
	called int
	ctxErr error // captures ctx.Err() at call time
}

func (c *dummyCloserCtx) Close(ctx context.Context) {
	c.called++
	c.ctxErr = ctx.Err()
}

type dummyCloserCtxErr struct {
	err    error
	called int
	ctxErr error // captures ctx.Err() at call time
}

func (c *dummyCloserCtxErr) Close(ctx context.Context) error {
	c.called++
	c.ctxErr = ctx.Err()
	return c.err
}

func Test_Closer_Cancellation(t *testing.T) {
	AssertRunnableRespectCancellation(t, CloserErr(&dummyCloser{}), time.Second)
	AssertRunnableRespectPreCancelledContext(t, CloserErr(&dummyCloser{}))
}

func Test_Closer_Close(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	closer := &dummyCloser{}

	err := CloserErr(closer).Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, closer.called)
}

func Test_Closer_Close_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	testErr := &dummyError{"dummy error"}
	closer := &dummyCloser{err: testErr}

	err := CloserErr(closer).Run(ctx)
	require.EqualError(t, err, "closer: Close() returned an error: dummy error")
	require.IsType(t, &RunnableError{}, err)
	require.True(t, errors.Is(err, testErr))

	require.Equal(t, 1, closer.called)
}

func Test_CloserCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	closer := &dummyCloserCtx{}

	err := CloserCtx(closer).Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, closer.called)
	require.NoError(t, closer.ctxErr, "context passed to Close should not be cancelled")
}

func Test_CloserCtxErr(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		closer := &dummyCloserCtxErr{}

		err := CloserCtxErr(closer).Run(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, closer.called)
		require.NoError(t, closer.ctxErr, "context passed to Close should not be cancelled")
	})

	t.Run("error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		testErr := &dummyError{"close failed"}
		closer := &dummyCloserCtxErr{err: testErr}

		err := CloserCtxErr(closer).Run(ctx)
		require.EqualError(t, err, "closer: Close() returned an error: close failed")
		require.True(t, errors.Is(err, testErr))
		require.Equal(t, 1, closer.called)
		require.NoError(t, closer.ctxErr, "context passed to Close should not be cancelled")
	})
}
