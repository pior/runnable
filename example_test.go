package runnable_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pior/runnable"
)

func NewJobs() *Jobs {
	return &Jobs{queue: make(chan string)}
}

type Jobs struct {
	queue chan string
}

func (s *Jobs) Perform(id string) {
	s.queue <- id
}

// Run executes enqueued jobs, drains the queue and quits.
func (s *Jobs) Run(ctx context.Context) error {
	for {
		select {
		case id := <-s.queue:
			fmt.Printf("Starting job %s\n", id)
			time.Sleep(time.Second)
			fmt.Printf("Completed job %s\n", id)

		default:
			if err := ctx.Err(); err != nil {
				close(s.queue)
				return err
			}
		}
	}
}

type CleanupTask struct{}

func (*CleanupTask) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func Example() {
	runnable.SetLogger(log.New(os.Stdout, "", 0))

	g := runnable.NewManager()

	jobs := NewJobs()
	g.Add(jobs)

	server := &http.Server{
		Addr: "127.0.0.1:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			jobs.Perform(id)
		}),
	}
	g.Add(runnable.HTTPServer(server), jobs)

	task := runnable.Func(func(ctx context.Context) error {
		_, _ = http.Post("http://127.0.0.1:8080/?id=1", "test/plain", nil)
		_, _ = http.Post("http://127.0.0.1:8080/?id=2", "test/plain", nil)
		_, _ = http.Post("http://127.0.0.1:8080/?id=3", "test/plain", nil)

		return nil // quit right away, will trigger a shutdown
	})
	g.Add(task)

	cleanup := runnable.Every(&CleanupTask{}, time.Hour)
	g.Add(cleanup, jobs)

	runnable.Run(g.Build())

	// INFO manager: runnable_test.Jobs started
	// INFO manager: runnable.httpServer started
	// INFO manager: func(runnable.RunnableFunc) started
	// INFO manager: periodic(runnable_test.CleanupTask) started
	// DBUG http_server: listening
	// Starting job 1
	// Completed job 1
	// Starting job 2
	// Completed job 2
	// Starting job 3
	// INFO manager: starting shutdown (func(runnable.RunnableFunc) died)
	// INFO manager: runnable.httpServer cancelled
	// INFO manager: func(runnable.RunnableFunc) cancelled
	// INFO manager: periodic(runnable_test.CleanupTask) cancelled
	// INFO manager: func(runnable.RunnableFunc) stopped
	// INFO manager: periodic(runnable_test.CleanupTask) stopped
	// DBUG http_server: shutdown (context cancelled)
	// INFO manager: runnable.httpServer stopped
	// INFO manager: runnable_test.Jobs cancelled
	// Completed job 3
	// INFO manager: runnable_test.Jobs stopped
	// INFO manager: shutdown complete
}
