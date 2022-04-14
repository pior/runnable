package runnable

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type ServerWithShutdown interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type httpServer struct {
	server          ServerWithShutdown
	shutdownTimeout time.Duration
}

// HTTPServer returns a runnable that runs a ServerWithShutdown (like *http.Server).
func HTTPServer(server ServerWithShutdown) Runnable {
	return &httpServer{server, time.Second * 30}
}

func (r *httpServer) Run(ctx context.Context) error {
	errChan := make(chan error)

	go func() {
		log.Debugf("http_server: listening")
		errChan <- r.server.ListenAndServe()
	}()

	var err error
	var shutdownErr error

	select {
	case <-ctx.Done():
		log.Debugf("http_server: shutdown (context cancelled)")
		shutdownErr = r.shutdown()
		err = <-errChan
	case err = <-errChan:
		log.Debugf("http_server: shutdown (error from http.Server)")
		shutdownErr = r.shutdown()
	}

	if err == http.ErrServerClosed {
		err = nil
	}
	if err == nil && shutdownErr != nil {
		err = fmt.Errorf("server shutdown: %w", shutdownErr)
	}

	return err
}

func (r *httpServer) shutdown() error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, r.shutdownTimeout)
	defer cancel()

	return r.server.Shutdown(ctx)
}
