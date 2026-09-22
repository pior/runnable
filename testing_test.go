package runnable

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func funcTesting(context.Context) error { return nil }

func newDummyRunnable() *dummyRunnable {
	return &dummyRunnable{}
}

type dummyRunnable struct{}

func (r *dummyRunnable) Run(ctx context.Context) error {
	logger.Info(runnableName(r) + ": started")
	<-ctx.Done()
	logger.Info(runnableName(r) + ": stopped")
	return ctx.Err()
}

func newCounterRunnable() *counter {
	return &counter{}
}

type counter struct {
	counter int
}

func (c *counter) Run(ctx context.Context) error {
	c.counter++
	return nil
}

func newDyingRunnable() *dyingRunnable {
	return &dyingRunnable{}
}

type dyingRunnable struct {
	counter int
}

func (r *dyingRunnable) Run(ctx context.Context) error {
	r.counter++
	return errors.New("dying")
}

type dummyError struct {
	message string
}

func (e *dummyError) Error() string {
	return e.message
}

func AssertRunnableRespectCancellation(t *testing.T, runnable Runnable, waitTime time.Duration) {
	t.Helper()

	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)

	errChan := make(chan error)

	go func() {
		errChan <- runnable.Run(ctx)
	}()

	cancelFunc()

	select {
	case <-time.After(waitTime):
		t.Fatal("did not return after " + waitTime.String())
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
	}
}

func AssertRunnableRespectPreCancelledContext(t *testing.T, runnable Runnable) {
	t.Helper()

	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	errChan := make(chan error)

	go func() {
		errChan <- runnable.Run(ctx)
	}()

	select {
	case <-time.After(time.Millisecond * 100):
		t.Fatal("did not return after 100ms")
	case err := <-errChan:
		if !errors.Is(err, context.Canceled) {
			require.NoError(t, err)
		}
	}
}

func AssertTimeout(t *testing.T, waitTime time.Duration, fn func()) {
	t.Helper()

	wait := make(chan bool)

	go func() {
		fn()
		close(wait)
	}()

	select {
	case <-time.After(waitTime):
		t.Fatal("timeout: " + waitTime.String())
	case <-wait:
	}
}

func cancelledContext() context.Context {
	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()
	return ctx
}

func initializeForExample() (context.Context, func()) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)

	SetLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))

	return ctx, cancel
}

// logBuffer is a concurrency-safe buffer for capturing log output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs redirects the package logger to a buffer, without timestamps,
// until the end of the test.
func captureLogs(t *testing.T) *logBuffer {
	t.Helper()

	previous := logger
	t.Cleanup(func() { SetLogger(previous) })

	buf := &logBuffer{}
	SetLogger(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))
	return buf
}
