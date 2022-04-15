# Runnable

[![GoDoc](https://godoc.org/github.com/pior/runnable?status.svg)](https://pkg.go.dev/github.com/pior/runnable?tab=doc)
[![Go Report Card](https://goreportcard.com/badge/github.com/pior/runnable)](https://goreportcard.com/report/github.com/pior/runnable)

Tooling to manage the execution of a process based on a `Runnable` interface:

```go
type Runnable interface {
	Run(context.Context) error
}
```

And a simpler `RunnableFunc` interface:

```go
type RunnableFunc func(context.Context) error
```

Example of an implementation of the command "yes":

```go
func main() {
	runnable.RunFunc(run)
}

func run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Println("y")
	}
}
```

## HTTP Server

```go
package main

import (
	"net/http"

	"github.com/pior/runnable"
)

func main() {
	server := &http.Server{
		Addr:    "127.0.0.1:8000",
		Handler: http.RedirectHandler("https://go.dev", http.StatusPermanentRedirect),
	}

	runnable.Run(runnable.HTTPServer(server))
}
```

## Manager: run multiple runnables with dependencies

The Manager starts and stops all runnables while respecting dependencies between them.
Components with dependencies will be stopped before their dependencies.

```go
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
	monitor = runnable.Periodic(runnable.PeriodicOptions{
		Period: 3 * time.Second,
	}, monitor)

	g := runnable.Manager(nil)
	g.Add(jobs)
	g.Add(serverRunner, jobs) // jobs is a dependency
	g.Add(monitor)

	runnable.Run(g.Build())
}
```

```go
package main

import (
	"context"
	"fmt"
	"sync/atomic"
)

type StupidJobQueue struct {
	queue    chan string
	executed int64
}

func NewStupidJobQueue() *StupidJobQueue {
	return &StupidJobQueue{queue: make(chan string)}
}

func (s *StupidJobQueue) Perform(url string) {
	s.queue <- url
}

func (s *StupidJobQueue) Executed() int64 {
	return atomic.LoadInt64(&s.executed)
}

func (s *StupidJobQueue) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case url := <-s.queue:
			fmt.Printf("Job from %s\n", url)
			atomic.AddInt64(&s.executed, 1)
		}
	}
}
```

Output:

```
$ go run ./cmd/example
[RUNNABLE] 2020/10/22 22:42:26 INFO manager: main.StupidJobQueue started
[RUNNABLE] 2020/10/22 22:42:26 INFO manager: runnable.httpServer started
[RUNNABLE] 2020/10/22 22:42:26 INFO manager: periodic(func(runnable.RunnableFunc)) started
Task executed: 0
Job from /
Job from /favicon.ico
Task executed: 2
^C[RUNNABLE] 2020/10/22 22:42:34 INFO signal: received signal interrupt
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: starting shutdown (context cancelled)
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: runnable.httpServer cancelled
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: periodic(func(runnable.RunnableFunc)) cancelled
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: periodic(func(runnable.RunnableFunc)) stopped
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: runnable.httpServer stopped
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: main.StupidJobQueue cancelled
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: main.StupidJobQueue stopped
[RUNNABLE] 2020/10/22 22:42:34 INFO manager: shutdown complete
```

## License

The MIT License (MIT)
