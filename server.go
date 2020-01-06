package runnable

import (
	"context"
	"fmt"
	"net/http"
)

type ServerWithShutdown interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type httpServer struct {
	server ServerWithShutdown
}

// HTTPServer returns a runnable that runs a ServerWithShutdown (like *http.Server).
func HTTPServer(server ServerWithShutdown) Runnable {
	return &httpServer{server}
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
		log.Debugf("http_server: shutdown for cancellation")
		shutdownErr = r.server.Shutdown(ctx)
		err = <-errChan
	case err = <-errChan:
		log.Debugf("http_server: shutdown for error")
		shutdownErr = r.server.Shutdown(ctx)
	}

	if err == http.ErrServerClosed {
		err = nil
	}
	if err == nil && shutdownErr != nil {
		err = fmt.Errorf("server shutdown: %w", shutdownErr)
	}

	return err
}
