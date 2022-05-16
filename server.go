package runnable

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type httpServer struct {
	server          *http.Server
	shutdownTimeout time.Duration
}

// HTTPServer returns a runnable that runs a *http.Server.
func HTTPServer(server *http.Server) Runnable {
	return &httpServer{server, time.Second * 30}
}

func (r *httpServer) Run(ctx context.Context) error {
	errChan := make(chan error)

	go func() {
		log.Printf("http_server: listening on %s", r.server.Addr)
		errChan <- r.server.ListenAndServe()
	}()

	var err error
	var shutdownErr error

	select {
	case <-ctx.Done():
		log.Printf("http_server: shutdown")
		shutdownErr = r.shutdown()
		err = <-errChan
	case err = <-errChan:
		log.Printf("http_server: shutdown (err: %s)", err)
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
