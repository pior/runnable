package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pior/runnable"
)

func main() {
	jobs := NewStupidJobQueue()

	server := &http.Server{
		Addr: "localhost:8000",
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			jobs.Perform(r.URL.Path)
			_, _ = fmt.Fprintln(rw, "Job enqueued!")
		}),
	}
	serverRunner := runnable.HTTPServer(server)

	monitor := runnable.Schedule(
		runnable.Func(func(ctx context.Context) error {
			fmt.Printf("Task executed: %d\n", jobs.Executed())
			return nil
		}),
		runnable.Every(3*time.Second),
	)

	// Shutdown takes at most 20s + 10s, below a 40s Kubernetes grace period.
	g := runnable.NewManager(
		runnable.ProcessShutdownTimeout(20*time.Second),
		runnable.ServiceShutdownTimeout(10*time.Second),
	)
	g.RegisterService(jobs)
	g.RegisterProcess(serverRunner)
	g.RegisterProcess(monitor)

	runnable.Run(g)
}
