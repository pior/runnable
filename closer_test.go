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

func Test_Closer_Cancellation(t *testing.T) {
	AssertRunnableRespectCancellation(t, Closer(&dummyCloser{}, nil), time.Second)
	AssertRunnableRespectPreCancelledContext(t, Closer(&dummyCloser{}, nil))
}

func Test_Closer_Name(t *testing.T) {
	AssertName(t, "closer(runnable.dummyCloser)", Closer(&dummyCloser{}, nil))
}

func Test_Closer_Close(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	closer := &dummyCloser{}

	err := Closer(closer, nil).Run(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, closer.called)
}

func Test_Closer_Close_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	testErr := &dummyError{"dummy error"}
	closer := &dummyCloser{err: testErr}

	err := Closer(closer, nil).Run(ctx)
	require.EqualError(t, err, "closer: Close() returned an error: dummy error")
	require.IsType(t, &RunnableError{}, err)
	require.True(t, errors.Is(err, testErr))

	require.Equal(t, 1, closer.called)
}
