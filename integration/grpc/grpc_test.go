// Package grpc checks runnable.GRPCServer against a real *grpc.Server. It is a
// separate module to keep grpc out of the runnable module dependencies.
package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/pior/runnable"
)

func TestGRPCServer(t *testing.T) {
	// start runs a gRPC server with the health service until the returned
	// cancel is called, and returns a client connected to it.
	start := func(t *testing.T, opts ...runnable.ServerOption) (healthpb.HealthClient, context.CancelFunc, <-chan error) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)

		server := grpc.NewServer()
		healthpb.RegisterHealthServer(server, health.NewServer())
		r := runnable.GRPCServer("", server, append(opts, runnable.Listener(ln))...)

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		done := make(chan error, 1)
		go func() { done <- r.Run(ctx) }()

		conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		return healthpb.NewHealthClient(conn), cancel, done
	}

	t.Run("drains on cancellation", func(t *testing.T) {
		client, cancel, done := start(t)

		_, err := client.Check(t.Context(), &healthpb.HealthCheckRequest{})
		require.NoError(t, err)

		cancel()
		require.NoError(t, <-done)
	})

	t.Run("stops when the drain times out", func(t *testing.T) {
		client, cancel, done := start(t, runnable.DrainTimeout(100*time.Millisecond))

		// Watch is a stream that never ends: it blocks GracefulStop.
		stream, err := client.Watch(t.Context(), &healthpb.HealthCheckRequest{})
		require.NoError(t, err)
		_, err = stream.Recv()
		require.NoError(t, err)

		cancel()
		select {
		case runErr := <-done:
			require.Equal(t, "server shutdown: context deadline exceeded", fmt.Sprint(runErr))
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return: Stop did not end GracefulStop")
		}

		_, err = stream.Recv()
		require.Error(t, err, "stream still open after Stop")
	})
}
