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
	// level=INFO msg=listening runnable=httpserver addr=127.0.0.1:8080
	// level=INFO msg=shutdown runnable=httpserver
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
	// level=INFO msg=listening runnable=httpserver addr=INVALID
	// level=INFO msg=shutdown runnable=httpserver error="listen tcp: address INVALID: missing port in address"
}
