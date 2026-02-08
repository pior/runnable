package runnable_test

import (
	"context"
	"fmt"
	"log/slog"
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
	runnable.SetLogger(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	g := runnable.Manager()

	jobs := NewJobs()
	g.RegisterService(jobs)

	server := &http.Server{
		Addr: "127.0.0.1:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get("id")
			jobs.Perform(id)
		}),
	}
	g.Register(runnable.HTTPServer(server))

	task := runnable.Func(func(ctx context.Context) error {
		_, _ = http.Post("http://127.0.0.1:8080/?id=1", "test/plain", nil)
		_, _ = http.Post("http://127.0.0.1:8080/?id=2", "test/plain", nil)
		_, _ = http.Post("http://127.0.0.1:8080/?id=3", "test/plain", nil)

		return nil // quit right away, will trigger a shutdown
	}).Name("enqueue")
	g.Register(task)

	cleanup := runnable.Schedule(&CleanupTask{}, runnable.Hourly())
	g.Register(cleanup)

	runnable.Run(g)

	// level=INFO msg="manager/Jobs: started"
	// level=INFO msg="manager/httpserver: started"
	// level=INFO msg="manager/enqueue: started"
	// level=INFO msg="manager/schedule/CleanupTask: started"
	// level=INFO msg="httpserver: listening" addr=127.0.0.1:8080
	// Starting job 1
	// Completed job 1
	// ...
	// level=INFO msg="manager: starting shutdown" reason="enqueue died"
	// level=INFO msg="manager: shutdown complete"
}
