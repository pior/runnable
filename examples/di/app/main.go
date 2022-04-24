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

	container.Provide(func() *runnable.App {
		return runnable.NewApp()
	})

	container.Provide(func(app *runnable.App) *di.DB {
		db := &di.DB{}
		app.Add(db, runnable.AppOrderDB)
		return db
	})

	container.Provide(func(app *runnable.App, db *di.DB) *di.JobQueue {
		q := di.NewJobQueue(db)
		app.Add(q, runnable.AppOrderDB)
		return q
	})

	container.Invoke(func(app *runnable.App, jobs *di.JobQueue, db *di.DB) {
		server := runnable.HTTPServer(newServer(jobs, db))
		app.Add(server, runnable.AppOrderDefault)

		monitor := runnable.Restart(&Task{jobs})
		app.Add(monitor, runnable.AppOrderDefault)

		runnable.Run(app)
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
