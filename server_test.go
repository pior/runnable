package runnable

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTPServer(t *testing.T) {
	t.Run("graceful shutdown", func(t *testing.T) {
		server := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		}

		// Use a real listener to get an available port.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		server.Addr = ln.Addr().String()
		_ = ln.Close()

		ctx, cancel := context.WithCancel(context.Background())

		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(server).Run(ctx)
		}()

		// Wait for the server to be accepting connections.
		require.Eventually(t, func() bool {
			conn, dialErr := net.Dial("tcp", server.Addr)
			if dialErr != nil {
				return false
			}
			_ = conn.Close()
			return true
		}, time.Second, 10*time.Millisecond)

		cancel()

		select {
		case runErr := <-errChan:
			require.NoError(t, runErr)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not shut down within 5s")
		}
	})

	t.Run("listener", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		server := &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("hello"))
			}),
		}

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(server, Listener(ln)).Run(ctx)
		}()

		resp, err := http.Get("http://" + ln.Addr().String())
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, "200 OK hello", resp.Status+" "+string(body))

		cancel()

		select {
		case runErr := <-errChan:
			require.NoError(t, runErr)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not shut down within 5s")
		}
	})

	t.Run("logs the drain", func(t *testing.T) {
		logs := captureLogs(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(&http.Server{Handler: http.NotFoundHandler()}, Listener(ln)).Run(ctx)
		}()

		require.Eventually(t, func() bool { return logs.String() != "" }, time.Second, time.Millisecond)
		cancel()
		require.NoError(t, <-errChan)

		require.Equal(t, `level=INFO msg="httpserver: listening" addr=`+ln.Addr().String()+"\n"+
			`level=INFO msg="httpserver: draining" timeout=5s`+"\n"+
			`level=INFO msg="httpserver: drained"`+"\n", logs.String())
	})

	t.Run("drain timeout", func(t *testing.T) {
		logs := captureLogs(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		inHandler := make(chan struct{})
		release := make(chan struct{})
		server := &http.Server{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				close(inHandler)
				<-release
			}),
		}

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(server, Listener(ln), DrainTimeout(50*time.Millisecond)).Run(ctx)
		}()

		go func() {
			resp, getErr := http.Get("http://" + ln.Addr().String())
			if getErr == nil {
				_ = resp.Body.Close()
			}
		}()
		<-inHandler

		cancel()
		require.EqualError(t, <-errChan, "server shutdown: context deadline exceeded")
		close(release)

		require.Contains(t, logs.String(), `level=INFO msg="httpserver: draining" timeout=50ms`+"\n"+
			`level=INFO msg="httpserver: drain timed out"`+"\n")
	})

	t.Run("drain failure", func(t *testing.T) {
		logs := captureLogs(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(&http.Server{Handler: http.NotFoundHandler()}, Listener(failingCloseListener{ln})).Run(ctx)
		}()

		require.Eventually(t, func() bool { return logs.String() != "" }, time.Second, time.Millisecond)
		cancel()
		require.EqualError(t, <-errChan, "server shutdown: close failed")

		// The error is returned, not logged.
		require.Contains(t, logs.String(), `level=INFO msg="httpserver: draining" timeout=5s`+"\n"+
			`level=INFO msg="httpserver: drain failed"`+"\n")
	})

	t.Run("logs the listening address once listening", func(t *testing.T) {
		logs := captureLogs(t)
		server := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		errChan := make(chan error, 1)
		go func() { errChan <- HTTPServer(server).Run(ctx) }()

		require.Eventually(t, func() bool { return logs.String() != "" }, time.Second, time.Millisecond)
		cancel()
		require.NoError(t, <-errChan)

		// The port picked by the system, not the configured :0.
		require.Regexp(t, `^level=INFO msg="httpserver: listening" addr=127\.0\.0\.1:[1-9][0-9]*\n`, logs.String())
	})

	t.Run("listen error", func(t *testing.T) {
		server := &http.Server{
			Addr:    "INVALID",
			Handler: http.NotFoundHandler(),
		}

		logs := captureLogs(t)

		err := HTTPServer(server).Run(context.Background())
		require.EqualError(t, err, "listen tcp: address INVALID: missing port in address")
		require.Empty(t, logs.String()) // never listened, and the manager logs the error
	})

	t.Run("pre-cancelled context", func(t *testing.T) {
		server := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		errChan := make(chan error, 1)
		go func() {
			errChan <- HTTPServer(server).Run(ctx)
		}()

		select {
		case err := <-errChan:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("server did not return within 5s")
		}
	})

	t.Run("name is configurable", func(t *testing.T) {
		server := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		}

		r := Named("api", HTTPServer(server))

		require.Equal(t, "api", runnableName(r))
	})

	t.Run("drain timeout is configurable", func(t *testing.T) {
		r := HTTPServer(&http.Server{}, DrainTimeout(10*time.Second)).(*server)

		require.Equal(t, fmt.Sprint(10*time.Second), fmt.Sprint(r.drainTimeout))
	})
}

// fakeGRPCServer has the methods of a *grpc.Server. Serve blocks until
// GracefulStop returns or Stop is called, GracefulStop blocks until release is
// closed or Stop is called.
type fakeGRPCServer struct {
	release chan struct{}
	stopped chan struct{}
	forced  chan struct{}
}

func newFakeGRPCServer() *fakeGRPCServer {
	return &fakeGRPCServer{
		release: make(chan struct{}),
		stopped: make(chan struct{}),
		forced:  make(chan struct{}),
	}
}

func (s *fakeGRPCServer) Serve(ln net.Listener) error {
	select {
	case <-s.stopped:
	case <-s.forced:
	}
	return ln.Close()
}

func (s *fakeGRPCServer) GracefulStop() {
	select {
	case <-s.release:
		close(s.stopped)
	case <-s.forced:
	}
}

func (s *fakeGRPCServer) Stop() { close(s.forced) }

func TestGRPCServer(t *testing.T) {
	listen := func(t *testing.T) net.Listener {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		return ln
	}

	t.Run("drains on cancellation", func(t *testing.T) {
		logs := captureLogs(t)
		ln := listen(t)
		s := newFakeGRPCServer()
		close(s.release)

		require.NoError(t, GRPCServer("", s, Listener(ln)).Run(cancelledContext()))
		require.Equal(t, `level=INFO msg="grpcserver: listening" addr=`+ln.Addr().String()+"\n"+
			`level=INFO msg="grpcserver: draining" timeout=5s`+"\n"+
			`level=INFO msg="grpcserver: drained"`+"\n", logs.String())
	})

	t.Run("stops when the drain times out", func(t *testing.T) {
		logs := captureLogs(t)
		s := newFakeGRPCServer()

		err := GRPCServer("", s, Listener(listen(t)), DrainTimeout(10*time.Millisecond)).Run(cancelledContext())
		require.EqualError(t, err, "server shutdown: context deadline exceeded")
		require.Contains(t, logs.String(), `level=INFO msg="grpcserver: drain timed out"`+"\n")
		require.True(t, isClosed(s.forced), "Stop not called")
	})

	t.Run("returns the serve error", func(t *testing.T) {
		s := &failingGRPCServer{err: errors.New("serve failed")}

		err := GRPCServer("127.0.0.1:0", s).Run(context.Background())
		require.EqualError(t, err, "serve failed")
	})

	t.Run("listen error", func(t *testing.T) {
		err := GRPCServer("INVALID", newFakeGRPCServer()).Run(context.Background())
		require.EqualError(t, err, "listen tcp: address INVALID: missing port in address")
	})
}

type failingGRPCServer struct{ err error }

func (s *failingGRPCServer) Serve(net.Listener) error { return s.err }
func (s *failingGRPCServer) GracefulStop()            {}
func (s *failingGRPCServer) Stop()                    {}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

type failingCloseListener struct{ net.Listener }

func (l failingCloseListener) Close() error {
	_ = l.Listener.Close()
	return errors.New("close failed")
}
