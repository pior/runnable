<!-- omit in toc -->
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

<!-- omit in toc -->
## Tools:

- [Process start and shutdown](#process-start-and-shutdown)
- [Restart](#restart)
- [HTTP Server](#http-server)
- [Manager](#manager)

### Process start and shutdown

To trigger a clean shutdown, a process must react to the termination signals (SIGINT, SIGTERM).

The `Run()` method is intended to be the process entrypoint:
- it immediately executes the runnable
- it cancels the context.Context when a **termination signal** is received
- it calls **log.Fatal with the error** if the runnable returned one

Example:
```go
func main() {
	runnable.Run(
		app.Build(),
	)
}
```

The `RunFunc()` method is also provided for convenience.

### Restart

The `Restart` runnable ensure that a component is running, even if it stops or crashes.

Example:
```go
func main() {
	runnable.Run(
		runnable.Restart(
			task.New(),
		),
	)
}
```

### HTTP Server

The `HTTPServer` runnable starts and gracefully shutdowns a `*http.Server`.

Example:
```go
func main() {
	server := &http.Server{
		Addr:    "127.0.0.1:8000",
		Handler: http.RedirectHandler("https://go.dev", 307),
	}

	runnable.Run(
		runnable.HTTPServer(server),
	)
}
```

### Manager

The `Manager` groups runnables into two tiers: **processes** (foreground work) and **services** (infrastructure).
On shutdown, processes are stopped first, then services.

```go
g := runnable.Manager()
g.AddService(jobQueue)
g.Add(httpServer)
g.Add(monitor)

runnable.Run(g)
```

<details>
  <summary markdown="span">Logs of a demo app</summary>

```shell
$ go run ./examples/example/
level=INFO msg=started runnable=manager/StupidJobQueue
level=INFO msg=started runnable=manager/httpserver
level=INFO msg=started runnable=manager/every-3s/RunnableFunc
...
^C
level=INFO msg="signal received" runnable=signal signal=interrupt
level=INFO msg="starting shutdown" runnable=manager reason="context cancelled"
level=INFO msg=stopped runnable=manager/httpserver
level=INFO msg=stopped runnable=manager/every-3s/RunnableFunc
level=INFO msg=stopped runnable=manager/StupidJobQueue
level=INFO msg="shutdown complete" runnable=manager
```

</details>

<!-- omit in toc -->
## License

The MIT License (MIT)
