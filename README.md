# Runnable

[![Build Status](https://github.com/pior/runnable/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/pior/runnable/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pior/runnable.svg)](https://pkg.go.dev/github.com/pior/runnable)
[![Go Report Card](https://goreportcard.com/badge/github.com/pior/runnable)](https://goreportcard.com/report/github.com/pior/runnable)

A Go library for orchestrating long-running processes with clean shutdown. Everything builds on a single interface:

```go
type Runnable interface {
    Run(context.Context) error
}
```

Shutdown is driven by context cancellation. When the context is cancelled, each runnable stops gracefully and returns.

## Manager

The `Manager` orchestrates multiple runnables with ordered shutdown. Runnables are organized in two tiers:

- **Processes** — the primary work (HTTP servers, workers, scheduled tasks)
- **Services** — infrastructure that processes depend on (databases, queues, metrics)

When shutdown is triggered (context cancelled or any runnable completes), processes are stopped first, then services. This ensures services remain available while processes drain.

```go
func main() {
    m := runnable.Manager()
    m.RegisterService(jobQueue)
    m.Register(runnable.HTTPServer(server))
    m.Register(monitor)

    runnable.Run(m)
}
```

A `Manager` is itself a `Runnable`, so managers can be nested for independent shutdown ordering.

<details>
  <summary>Example logs</summary>

```
$ go run ./examples/example/
level=INFO msg=started runnable=manager/StupidJobQueue
level=INFO msg=started runnable=manager/httpserver
level=INFO msg=listening runnable=httpserver addr=localhost:8000
...
^C
level=INFO msg="received signal" runnable=signal/manager signal=interrupt
level=INFO msg="starting shutdown" runnable=manager reason="context cancelled"
level=INFO msg="shutting down" runnable=httpserver
level=INFO msg=stopped runnable=httpserver
level=INFO msg=stopped runnable=manager/httpserver
level=INFO msg=stopped runnable=manager/StupidJobQueue
level=INFO msg="shutdown complete" runnable=manager
```

</details>

## Entrypoints

`Run`, `RunFunc`, and `RunGroup` are intended as `main()` helpers. They handle OS signals (SIGINT/SIGTERM) and call `log.Fatal` on error.

```go
func main() {
    runnable.Run(myApp)
}
```

## Wrappers

Wrappers compose behavior around a `Runnable`:

| Wrapper | Description |
|---------|-------------|
| `HTTPServer(server)` | Start and gracefully shut down a `*http.Server` |
| `Restart(r, opts...)` | Auto-restart on failure, with configurable limits and delays |
| `Every(r, duration)` | Run periodically |
| `Recover(r)` | Catch panics and return them as errors |
| `Signal(r, signals...)` | Cancel context on OS signals |
| `Closer(c)` | Call `Close()` on context cancellation |
| `Func(fn)` | Adapt a `func(context.Context) error` to `Runnable` |

## License

The MIT License (MIT)
