package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pior/runnable"
)

func monitor(jobs *StupidJobQueue) runnable.RunnableFunc {
	return func(ctx context.Context) error {
		fmt.Printf("Task executed: %d\t\ttasks waiting: %d\n", jobs.Executed(), jobs.Waiting())
		return nil
	}
}

func main() {
	jobs := NewStupidJobQueue()

	// curl http://localhost:8000/?count=3
	server := &http.Server{
		Addr: "localhost:8000",
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			count := r.Form.Get("count")
			if count == "" {
				count = "1"
			}
			countInt, _ := strconv.Atoi(count)

			for i := 0; i < countInt; i++ {
				jobs.Enqueue(r.URL.Path)
				fmt.Fprintln(rw, "Job enqueued!")
			}
		}),
	}
	serverRunner := runnable.HTTPServer(server)

	monitor := runnable.Every(runnable.Func(monitor(jobs)), 2*time.Second)

	c := runnable.Component().WithShutdownTimeout(5*time.Second, 3*time.Second)
	c.Add(serverRunner)
	c.AddService(jobs)
	c.Add(monitor)

	fmt.Println("Enqueue jobs with: curl http://localhost:8000/?count=3")

	runnable.Run(c)
}
