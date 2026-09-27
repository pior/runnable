package runnable_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/pior/runnable"
)

// Cache is a service that the server depends on.
type Cache struct{}

func (*Cache) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func ExampleHTTPServer() {
	server := &http.Server{
		Addr:    "127.0.0.1:18080",
		Handler: http.NotFoundHandler(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	fmt.Println(runnable.HTTPServer(server).Run(ctx))

	// Output:
	// level=INFO msg="httpserver: listening" addr=127.0.0.1:18080
	// level=INFO msg="httpserver: draining" timeout=5s
	// level=INFO msg="httpserver: drained"
	// <nil>
}

func ExampleHTTPServer_options() {
	// The listening address is random, keep it out of the example output.
	runnable.SetLogger(slog.New(slog.DiscardHandler))
	defer runnable.SetLogger(nil)

	// Listener serves on a listener opened by the caller: port 0 in tests, a unix
	// socket, a TLS listener, or a socket passed by the service manager.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hello")
		}),
	}

	r := runnable.HTTPServer(server,
		runnable.Listener(ln),
		runnable.DrainTimeout(10*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- r.Run(ctx) }()

	resp, err := http.Get("http://" + ln.Addr().String())
	if err != nil {
		panic(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	fmt.Println(string(body))

	cancel()
	fmt.Println(<-done)

	// Output:
	// hello
	// <nil>
}

func ExampleHTTPServer_manager() {
	server := &http.Server{
		Addr:    "127.0.0.1:18080",
		Handler: http.NotFoundHandler(),
	}

	// The manager stops the server first, draining in-flight requests, then the
	// job queue it depends on.
	m := runnable.NewManager()
	m.RegisterService(&Cache{})
	m.RegisterProcess(runnable.HTTPServer(server))

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	fmt.Println(m.Run(ctx))

	// Output:
	// level=INFO msg="manager/Cache: started"
	// level=INFO msg="manager/httpserver: started"
	// level=INFO msg="manager/httpserver: listening" addr=127.0.0.1:18080
	// level=INFO msg="manager: starting shutdown" reason="context cancelled"
	// level=INFO msg="manager/httpserver: draining" timeout=5s
	// level=INFO msg="manager/httpserver: drained"
	// level=INFO msg="manager/httpserver: stopped"
	// level=INFO msg="manager/Cache: stopped"
	// level=INFO msg="manager: shutdown complete"
	// <nil>
}

func ExampleHTTPServer_error() {
	server := &http.Server{
		Addr:    "INVALID",
		Handler: http.NotFoundHandler(),
	}

	fmt.Println(runnable.HTTPServer(server).Run(context.Background()))

	// Output:
	// listen tcp: address INVALID: missing port in address
}

// GreeterServer stands in for a *grpc.Server, to keep grpc out of the examples.
type GreeterServer struct{ stopped chan struct{} }

func (s *GreeterServer) Serve(ln net.Listener) error {
	<-s.stopped
	return ln.Close()
}

func (s *GreeterServer) GracefulStop() { close(s.stopped) }

func (s *GreeterServer) Stop() {}

func ExampleGRPCServer() {
	server := &GreeterServer{stopped: make(chan struct{})} // grpc.NewServer()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	fmt.Println(runnable.GRPCServer("127.0.0.1:18081", server).Run(ctx))

	// Output:
	// level=INFO msg="grpcserver: listening" addr=127.0.0.1:18081
	// level=INFO msg="grpcserver: draining" timeout=5s
	// level=INFO msg="grpcserver: drained"
	// <nil>
}
