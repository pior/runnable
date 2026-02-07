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

	monitor := runnable.Func(func(ctx context.Context) error {
		fmt.Printf("Task executed: %d\n", jobs.Executed())
		return nil
	})
	monitor = runnable.Every(monitor, 3*time.Second)

	g := runnable.Manager()
	g.AddService(jobs)
	g.Add(serverRunner)
	g.Add(monitor)

	runnable.Run(g)
}
