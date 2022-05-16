package runnable_test

import (
	"context"
	stdlog "log"
	"net/http"
	"os"
	"time"

	"github.com/pior/runnable"
)

func ExampleHTTPServer() {
	runnable.SetLogger(stdlog.New(os.Stdout, "", 0))

	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: http.NotFoundHandler(),
	}

	r := runnable.HTTPServer(server)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()

	_ = r.Run(ctx)

	// Output:
	// http_server: listening on 127.0.0.1:8080
	// http_server: shutdown
}

func ExampleHTTPServer_error() {
	runnable.SetLogger(stdlog.New(os.Stdout, "", 0))

	server := &http.Server{
		Addr:    "INVALID",
		Handler: http.NotFoundHandler(),
	}

	r := runnable.HTTPServer(server)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*1000)
	defer cancel()

	_ = r.Run(ctx)

	// Output:
	// http_server: listening on INVALID
	// http_server: shutdown (err: listen tcp: address INVALID: missing port in address)
}
