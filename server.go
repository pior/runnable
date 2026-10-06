package runnable

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

type server struct {
	name         string
	addr         func() string
	serve        func(net.Listener) error
	shutdown     func(context.Context) error
	listener     net.Listener
	drainTimeout time.Duration
}

var _ Runnable = (*server)(nil)

func (r *server) runnableName() string { return r.name }

// HTTPServer returns a runnable that runs a [*http.Server].
//
// On context cancellation, it calls [http.Server.Shutdown] to gracefully drain
// in-flight requests before returning, for at most [DrainTimeout], 5 seconds by
// default. The server listens on [http.Server.Addr] unless [Listener] is set.
//
//	runnable.HTTPServer(server, runnable.DrainTimeout(10*time.Second))
func HTTPServer(s *http.Server, opts ...ServerOption) Runnable {
	// Read at Run, so an Addr set after HTTPServer is called is used.
	addr := func() string {
		if s.Addr == "" {
			return ":http"
		}
		return s.Addr
	}
	serve := func(ln net.Listener) error {
		if err := s.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	return newServer("httpserver", addr, serve, s.Shutdown, opts)
}

// GRPCServer returns a runnable that runs a *grpc.Server, or any server with
// the same Serve, GracefulStop and Stop methods. It listens on addr unless
// [Listener] is set.
//
// On context cancellation, it calls GracefulStop to drain in-flight calls
// before returning, for at most [DrainTimeout], 5 seconds by default. If the
// drain times out, it calls Stop to close the remaining connections.
//
//	runnable.GRPCServer(":50051", grpc.NewServer(), runnable.DrainTimeout(10*time.Second))
func GRPCServer(addr string, s interface {
	Serve(net.Listener) error
	GracefulStop()
	Stop()
}, opts ...ServerOption,
) Runnable {
	shutdown := func(ctx context.Context) error {
		stopped := make(chan struct{})
		go func() {
			s.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
			return nil
		case <-ctx.Done():
			s.Stop()
			return ctx.Err()
		}
	}
	return newServer("grpcserver", func() string { return addr }, s.Serve, shutdown, opts)
}

func newServer(
	name string,
	addr func() string,
	serve func(net.Listener) error,
	shutdown func(context.Context) error,
	opts []ServerOption,
) *server {
	r := &server{
		name:         name,
		addr:         addr,
		serve:        serve,
		shutdown:     shutdown,
		drainTimeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt.applyServer(r)
	}
	return r
}

// ServerOption configures [HTTPServer] and [GRPCServer].
type ServerOption interface{ applyServer(*server) }

var (
	_ ServerOption = drainTimeout(0)
	_ ServerOption = listener{}
)

// DrainTimeout sets the maximum time allowed for the graceful shutdown of a
// [HTTPServer] or [GRPCServer]. Defaults to 5 seconds. Under a [Manager], keep it
// below [ProcessShutdownTimeout].
func DrainTimeout(d time.Duration) ServerOption {
	return drainTimeout(d)
}

type drainTimeout time.Duration

func (d drainTimeout) applyServer(r *server) { r.drainTimeout = time.Duration(d) }

// Listener makes a [HTTPServer] or [GRPCServer] accept connections on ln instead of
// listening on its address. Use it to listen on port 0 in tests, on a unix
// socket, on a TLS listener, or on a socket passed by the service manager
// (socket activation). The server takes ownership of ln and closes it on
// shutdown.
func Listener(ln net.Listener) ServerOption {
	return listener{ln}
}

type listener struct{ ln net.Listener }

func (l listener) applyServer(r *server) { r.listener = l.ln }

func (r *server) Run(ctx context.Context) error {
	name := resolveName(ctx, r.name)

	ln, err := r.listen(ctx)
	if err != nil {
		return err
	}
	logger.Info(name+": listening", "addr", ln.Addr().String())

	errChan := make(chan error, 1)
	go func() { errChan <- r.serve(ln) }()

	select {
	case err = <-errChan:
		// Server stopped on its own, no shutdown needed.
		return err
	case <-ctx.Done():
	}

	// Serve may have failed as ctx was cancelled: there is nothing to drain.
	select {
	case err = <-errChan:
		return err
	default:
	}

	logger.Info(name+": draining", "timeout", r.drainTimeout)
	shutdownErr := r.drain()
	err = <-errChan

	switch {
	case errors.Is(shutdownErr, context.DeadlineExceeded):
		logger.Info(name + ": drain timed out")
	case shutdownErr != nil:
		logger.Info(name + ": drain failed") // the cause is in the returned error
	default:
		logger.Info(name + ": drained")
	}

	if err != nil {
		return err
	}
	if shutdownErr != nil {
		return fmt.Errorf("server shutdown: %w", shutdownErr)
	}
	return nil
}

// listen returns the listener set with [Listener], or listens on the address.
func (r *server) listen(ctx context.Context) (net.Listener, error) {
	if r.listener != nil {
		return r.listener, nil
	}
	// Listen even when ctx is already cancelled: Run then drains and stops cleanly.
	var lc net.ListenConfig
	return lc.Listen(context.WithoutCancel(ctx), "tcp", r.addr())
}

func (r *server) drain() error {
	ctx := context.Background() // only used for timeout in shutdown.
	ctx, cancel := context.WithTimeout(ctx, r.drainTimeout)
	defer cancel()

	return r.shutdown(ctx)
}
