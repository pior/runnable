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
	runnable.SetStandardLogger(log.New(os.Stdout, "", 0))

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	g := runnable.Manager(nil)

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

		cancel() // simulate a shutdown

		<-ctx.Done()
		return nil
	})
	g.Add(task)

	periodicCleanup := runnable.Periodic(
		runnable.PeriodicOptions{Period: time.Hour},
		&CleanupTask{},
	)
	g.Add(periodicCleanup, jobs)

	runnable.Run(g.Build())
}
