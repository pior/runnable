package runnable

import (
	"context"
	"errors"
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
// in-flight requests before returning. The shutdown timeout defaults to 5 seconds,
// half of the [Manager] default shutdown timeout,
// and can be configured with [httpServer.ShutdownTimeout].
func HTTPServer(server *http.Server) *httpServer {
	return &httpServer{
		name:            "httpserver",
		server:          server,
		shutdownTimeout: 5 * time.Second,
	}
}

// ShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Defaults to 5 seconds.
func (r *httpServer) ShutdownTimeout(dur time.Duration) *httpServer {
	r.shutdownTimeout = dur
	return r
}

func (r *httpServer) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)
	errChan := make(chan error)

	go func() {
		logger.Info(name+": listening", "addr", r.server.Addr)
		errChan <- r.server.ListenAndServe()
	}()

	var err error
	var shutdownErr error

	select {
	case <-ctx.Done():
		logger.Info(name + ": shutting down")
		shutdownErr = r.shutdown()
		err = <-errChan
		logger.Info(name + ": stopped")
	case err = <-errChan:
		logger.Info(name+": stopped with error", "error", err)
		// Server stopped on its own — no Shutdown needed.
	}

	if errors.Is(err, http.ErrServerClosed) {
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
