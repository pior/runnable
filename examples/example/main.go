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
			fmt.Fprintln(rw, "Job enqueued!")
		}),
	}
	serverRunner := runnable.HTTPServer(server)

	monitor := runnable.Func(func(ctx context.Context) error {
		fmt.Printf("Task executed: %d\n", jobs.Executed())
		return nil
	})
	monitor = runnable.Every(monitor, 3*time.Second)

	g := runnable.NewManager()
	g.Add(jobs)
	g.Add(serverRunner, jobs) // jobs is a dependency
	g.Add(monitor)

	runnable.Run(g.Build())
}
