package runnable

import (
	"net/http"
)

func ExampleHTTPServer() {
	ctx, cancel := initializeForExample()
	defer cancel()

	server := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: http.NotFoundHandler(),
	}

	r := HTTPServer(server)

	_ = r.Run(ctx)

	// Output:
	// httpserver: listening on 127.0.0.1:8080
	// httpserver: shutdown
}

func ExampleHTTPServer_error() {
	ctx, cancel := initializeForExample()
	defer cancel()

	server := &http.Server{
		Addr:    "INVALID",
		Handler: http.NotFoundHandler(),
	}

	r := HTTPServer(server)

	_ = r.Run(ctx)

	// Output:
	// httpserver: listening on INVALID
	// httpserver: shutdown (err: listen tcp: address INVALID: missing port in address)
}
