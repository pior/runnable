package runnable

import (
	"context"
	"fmt"
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

	t.Run("listen error", func(t *testing.T) {
		server := &http.Server{
			Addr:    "INVALID",
			Handler: http.NotFoundHandler(),
		}

		err := HTTPServer(server).Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing port in address")
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

		r := HTTPServer(server).Name("api")

		require.Equal(t, "api", r.runnableName())
	})

	t.Run("shutdown timeout is configurable", func(t *testing.T) {
		server := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		}

		r := HTTPServer(server).ShutdownTimeout(5 * time.Second)

		require.Equal(t, fmt.Sprint(5*time.Second), fmt.Sprint(r.shutdownTimeout))
	})
}

func ExampleHTTPServer() {
	ctx, cancel := initializeForExample()
	defer cancel()

	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: http.NotFoundHandler(),
	}

	r := HTTPServer(server)

	_ = r.Run(ctx)

	// Output:
	// level=INFO msg="httpserver: listening" addr=127.0.0.1:8080
	// level=INFO msg="httpserver: shutting down"
	// level=INFO msg="httpserver: stopped"
}

func ExampleHTTPServer_error() {
	ctx, cancel := initializeForExample()
	defer cancel()

	server := &http.Server{
		Addr:    "INVALID",
		Handler: http.NotFoundHandler(),
	}

	r := HTTPServer(server)

	_ = r.Run(ctx)

	// Output:
	// level=INFO msg="httpserver: listening" addr=INVALID
	// level=INFO msg="httpserver: stopped with error" error="listen tcp: address INVALID: missing port in address"
}
