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
		opt.applyHTTPServer(r)
	}
	return r
}

// HTTPServerOption configures [HTTPServer].
type HTTPServerOption interface{ applyHTTPServer(*httpServer) }

type httpServerOptionFunc func(*httpServer)

func (f httpServerOptionFunc) applyHTTPServer(r *httpServer) { f(r) }

// DrainTimeout sets the maximum time allowed for graceful shutdown of an
// [HTTPServer]. Defaults to 5 seconds. Under a [Manager], keep it below
// [ProcessShutdownTimeout].
func DrainTimeout(d time.Duration) HTTPServerOption {
	return httpServerOptionFunc(func(r *httpServer) { r.shutdownTimeout = d })
}

// Listener makes an [HTTPServer] accept connections on ln instead of listening
// on [http.Server.Addr]. Use it to listen on port 0 in tests, on a unix socket,
// on a TLS listener, or on a socket passed by the service manager (socket
// activation). The server takes ownership of ln and closes it on shutdown.
func Listener(ln net.Listener) HTTPServerOption {
	return httpServerOptionFunc(func(r *httpServer) { r.listener = ln })
}

func (r *httpServer) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)

	ln, err := r.listen(ctx)
	if err != nil {
		return err
	}
	logger.Info(name+": listening", "addr", ln.Addr().String())

	errChan := make(chan error, 1)
	go func() { errChan <- r.server.Serve(ln) }()

	select {
	case err = <-errChan:
		// Server stopped on its own, no Shutdown needed.
		return ignoreServerClosed(err)
	case <-ctx.Done():
	}

	// Serve may have failed as ctx was cancelled: there is nothing to drain.
	select {
	case err = <-errChan:
		return ignoreServerClosed(err)
	default:
	}

	logger.Info(name+": draining", "timeout", r.shutdownTimeout)
	shutdownErr := r.shutdown()
	err = <-errChan

	switch {
	case errors.Is(shutdownErr, context.DeadlineExceeded):
		logger.Info(name + ": drain timed out")
	case shutdownErr != nil:
		logger.Info(name + ": drain failed") // the cause is in the returned error
	default:
		logger.Info(name + ": drained")
	}

	if err = ignoreServerClosed(err); err != nil {
		return err
	}
	if shutdownErr != nil {
		return fmt.Errorf("server shutdown: %w", shutdownErr)
	}
	return nil
}

func ignoreServerClosed(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// listen returns the listener set with [Listener], or listens on
// [http.Server.Addr] like [http.Server.ListenAndServe].
func (r *httpServer) listen(ctx context.Context) (net.Listener, error) {
	if r.listener != nil {
		return r.listener, nil
	}
	addr := r.server.Addr
	if addr == "" {
		addr = ":http"
	}
	// Listen even when ctx is already cancelled: Run then drains and stops cleanly.
	var lc net.ListenConfig
	return lc.Listen(context.WithoutCancel(ctx), "tcp", addr)
}

func (r *httpServer) shutdown() error {
	ctx := context.Background() // only used for timeout in Shutdown.
	ctx, cancel := context.WithTimeout(ctx, r.shutdownTimeout)
	defer cancel()

	return r.server.Shutdown(ctx)
}
