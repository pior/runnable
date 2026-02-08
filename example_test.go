package runnable_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pior/runnable"
)

// exampleLogger returns a text logger writing to stdout without timestamps,
// suitable for deterministic testable examples.
func exampleLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
}

// JobQueue is a long-running service that processes background jobs.
type JobQueue struct{}

func (q *JobQueue) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (q *JobQueue) Enqueue(job string) {
	fmt.Println("JobQueue: " + job)
}

// CleanupTask enqueues a cleanup job on each execution.
type CleanupTask struct {
	jobs *JobQueue
	runs int
	done chan struct{}
}

func (t *CleanupTask) Run(_ context.Context) error {
	t.runs++
	t.jobs.Enqueue(fmt.Sprintf("cleanup-%d", t.runs))
	if t.runs >= 3 {
		close(t.done)
	}
	return nil
}

func Example() {
	runnable.SetLogger(exampleLogger())

	jobs := &JobQueue{}
	done := make(chan struct{})
	cleanup := &CleanupTask{jobs: jobs, done: done}

	m := runnable.Manager()
	m.RegisterService(jobs)
	m.Register(runnable.Schedule(cleanup, runnable.Every(500*time.Millisecond)))
	m.Register(runnable.Func(func(_ context.Context) error {
		<-done
		return nil
	}).Name("app"))

	runnable.Run(m)

	// Output:
	// level=INFO msg="manager/JobQueue: started"
	// level=INFO msg="manager/schedule/CleanupTask: started"
	// level=INFO msg="manager/app: started"
	// JobQueue: cleanup-1
	// JobQueue: cleanup-2
	// JobQueue: cleanup-3
	// level=INFO msg="manager/app: stopped"
	// level=INFO msg="manager: starting shutdown" reason="app died"
	// level=INFO msg="manager/schedule/CleanupTask: stopped"
	// level=INFO msg="manager/JobQueue: stopped"
	// level=INFO msg="manager: shutdown complete"
}
