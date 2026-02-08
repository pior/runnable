package runnable

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type httpServer struct {
	name            string
	server          *http.Server
	shutdownTimeout time.Duration
}

var _ Runnable = (*httpServer)(nil)

func (r *httpServer) runnableName() string { return r.name }

// HTTPServer returns a runnable that runs a [*http.Server].
//
// On context cancellation, it calls [http.Server.Shutdown] to gracefully drain
// in-flight requests before returning. The shutdown timeout defaults to 30 seconds
// and can be configured with [httpServer.ShutdownTimeout].
func HTTPServer(server *http.Server) *httpServer {
	return &httpServer{
		name:            "httpserver",
		server:          server,
		shutdownTimeout: 30 * time.Second,
	}
}

// Name sets the runnable name, used in log messages. Defaults to "httpserver".
func (r *httpServer) Name(name string) *httpServer {
	r.name = name
	return r
}

// ShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Defaults to 30 seconds.
func (r *httpServer) ShutdownTimeout(dur time.Duration) *httpServer {
	r.shutdownTimeout = dur
	return r
}

func (r *httpServer) Run(ctx context.Context) error {
	errChan := make(chan error)

	go func() {
		logger.Info("listening", "runnable", r.name, "addr", r.server.Addr)
		errChan <- r.server.ListenAndServe()
	}()

	var err error
	var shutdownErr error

	select {
	case <-ctx.Done():
		logger.Info("shutting down", "runnable", r.name)
		shutdownErr = r.shutdown()
		err = <-errChan
		logger.Info("stopped", "runnable", r.name)
	case err = <-errChan:
		logger.Info("stopped with error", "runnable", r.name, "error", err)
		// Server stopped on its own — no Shutdown needed.
	}

	if err == http.ErrServerClosed {
		err = nil
	}
	if err != nil {
		return err
	}
	if shutdownErr != nil {
		return fmt.Errorf("server shutdown: %w", shutdownErr)
	}
	return nil
}

func (r *httpServer) shutdown() error {
	ctx := context.Background() // only used for timeout in Shutdown.
	ctx, cancel := context.WithTimeout(ctx, r.shutdownTimeout)
	defer cancel()

	return r.server.Shutdown(ctx)
}
