package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pior/runnable"
	"github.com/pior/runnable/examples/jobqueue"
)

// Run it with:
// go run ./examples/component/main.go

// Test it with:
// curl "http://localhost:8000/?count=3"

func jobMonitor(jobs *jobqueue.JobQueue) runnable.RunnableFunc {
	return func(ctx context.Context) error {
		fmt.Printf("Task executed: %d\t\ttasks waiting: %d\n", jobs.Executed(), jobs.Waiting())
		return nil
	}
}

func main() {
	jobs := jobqueue.New()

	server := &http.Server{
		Addr: "localhost:8000",
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			jobs.Enqueue(r.URL.Path)
			fmt.Fprintln(rw, "Job enqueued!")
		}),
	}

	c := runnable.Component().
		WithProcessShutdownTimeout(5 * time.Second).
		WithServiceShutdownTimeout(5 * time.Second).
		AddProcess(runnable.HTTPServer(server)).
		AddService(jobs).
		AddProcess(
			runnable.Every(runnable.Func(jobMonitor(jobs)), 2*time.Second),
		)

	fmt.Println("Enqueue jobs with: curl http://localhost:8000/?count=3")

	runnable.Run(c)
}
