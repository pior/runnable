package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pior/runnable"
	"github.com/pior/runnable/examples/di"
	"go.uber.org/dig"
)

func main() {
	container := dig.New()

	container.Provide(func() runnable.AppManager {
		return runnable.NewManager()
	})

	container.Provide(func(app runnable.AppManager) *di.DB {
		db := &di.DB{}
		app.Add(db) // should be wrapped with a runnable.Closer()
		return db
	})

	container.Provide(func(app runnable.AppManager, db *di.DB) *di.JobQueue {
		q := di.NewJobQueue(db)
		app.Add(q, db)
		return q
	})

	container.Invoke(func(app runnable.AppManager, jobs *di.JobQueue, db *di.DB) {
		server := newServer(jobs, db)
		app.Add(runnable.HTTPServer(server), jobs, db)

		monitor := runnable.Restart(&Task{jobs})
		app.Add(monitor, jobs)

		runnable.Run(app.Build())
	})
}

func newServer(jobs *di.JobQueue, db *di.DB) *http.Server {
	return &http.Server{
		Addr: "localhost:8000",
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			jobs.Perform(r.URL.Path)
			db.Read()
			fmt.Fprintln(rw, "Job enqueued!")
		}),
	}
}

type Task struct {
	jobs *di.JobQueue
}

func (t *Task) Run(_ context.Context) error {
	http.Post("http://localhost:8000", "text/plain", nil)

	time.Sleep(time.Second * 2)
	fmt.Printf("Task executed: %d\n", t.jobs.Executed())
	return nil
}
