package runnable

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Server is a runnable that runs a [*http.Server]. Build it with [HTTPServer].
//
// On context cancellation, it calls [http.Server.Shutdown] to gracefully drain
// in-flight requests before returning, for at most [Server.ShutdownTimeout].
type Server struct {
	name            string
	server          *http.Server
	listener        net.Listener
	shutdownTimeout time.Duration
}

var _ Runnable = (*Server)(nil)

func (r *Server) runnableName() string { return r.name }

// HTTPServer returns a [Server] running server, listening on [http.Server.Addr]
// unless [Server.Listener] is set.
//
//	runnable.HTTPServer(server).ShutdownTimeout(10 * time.Second)
func HTTPServer(server *http.Server) *Server {
	return &Server{
		name:            "httpserver",
		server:          server,
		shutdownTimeout: 5 * time.Second,
	}
}

// ShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Defaults to 5 seconds. Under a [Manager], keep it below the process phase of
// the manager's shutdown budget.
func (r *Server) ShutdownTimeout(dur time.Duration) *Server {
	r.shutdownTimeout = dur
	return r
}

// Listener makes the server accept connections on ln instead of listening on
// [http.Server.Addr]. Use it to listen on port 0 in tests, on a unix socket, on a
// TLS listener, or on a socket passed by the service manager (socket activation).
// The server takes ownership of ln and closes it on shutdown.
func (r *Server) Listener(ln net.Listener) *Server {
	r.listener = ln
	return r
}

func (r *Server) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)
	errChan := make(chan error)

	go func() {
		if r.listener != nil {
			logger.Info(name+": listening", "addr", r.listener.Addr().String())
			errChan <- r.server.Serve(r.listener)
			return
		}
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

func (r *Server) shutdown() error {
	ctx := context.Background() // only used for timeout in Shutdown.
	ctx, cancel := context.WithTimeout(ctx, r.shutdownTimeout)
	defer cancel()

	return r.server.Shutdown(ctx)
}
