package runnable

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type httpServer struct {
	name            string
	server          *http.Server
	listener        net.Listener
	shutdownTimeout time.Duration
}

var _ Runnable = (*httpServer)(nil)

func (r *httpServer) runnableName() string { return r.name }

// HTTPServer returns a runnable that runs a [*http.Server].
//
// On context cancellation, it calls [http.Server.Shutdown] to gracefully drain
// in-flight requests before returning, for at most [DrainTimeout], 5 seconds by
// default. The server listens on [http.Server.Addr] unless [Listener] is set.
//
//	runnable.HTTPServer(server, runnable.DrainTimeout(10*time.Second))
func HTTPServer(server *http.Server, opts ...HTTPServerOption) Runnable {
	r := &httpServer{
		name:            "httpserver",
		server:          server,
		shutdownTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// HTTPServerOption configures [HTTPServer].
type HTTPServerOption func(*httpServer)

// DrainTimeout sets the maximum time allowed for graceful shutdown of an
// [HTTPServer]. Defaults to 5 seconds. Under a [Manager], keep it below the
// process phase of the manager's shutdown budget.
func DrainTimeout(d time.Duration) HTTPServerOption {
	return func(r *httpServer) { r.shutdownTimeout = d }
}

// Listener makes an [HTTPServer] accept connections on ln instead of listening
// on [http.Server.Addr]. Use it to listen on port 0 in tests, on a unix socket,
// on a TLS listener, or on a socket passed by the service manager (socket
// activation). The server takes ownership of ln and closes it on shutdown.
func Listener(ln net.Listener) HTTPServerOption {
	return func(r *httpServer) { r.listener = ln }
}

func (r *httpServer) Run(ctx context.Context) error {
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

func (r *httpServer) shutdown() error {
	ctx := context.Background() // only used for timeout in Shutdown.
	ctx, cancel := context.WithTimeout(ctx, r.shutdownTimeout)
	defer cancel()

	return r.server.Shutdown(ctx)
}
