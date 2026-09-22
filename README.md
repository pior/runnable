# Runnable

[![Build Status](https://github.com/pior/runnable/actions/workflows/go.yml/badge.svg?branch=master)](https://github.com/pior/runnable/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pior/runnable.svg)](https://pkg.go.dev/github.com/pior/runnable)
[![Go Report Card](https://goreportcard.com/badge/github.com/pior/runnable)](https://goreportcard.com/report/github.com/pior/runnable)

A zero-dependency Go library for orchestrating long-running processes with clean shutdown. Everything builds on a single interface:

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
    m := runnable.NewManager()
    m.RegisterService(jobQueue)
    m.RegisterProcess(runnable.HTTPServer(server))
    m.RegisterProcess(monitor)

    runnable.Run(m)
}
```

A `Manager` is itself a `Runnable`, so managers can be nested for independent shutdown ordering.

<details>
  <summary>Example logs</summary>

```
$ go run ./examples/example/
INFO manager/StupidJobQueue: started
INFO manager/httpserver: started
INFO manager/schedule/main.main.func2: started
INFO manager/httpserver: listening addr=localhost:8000
...
^C
INFO signal/manager: received signal signal=interrupt
INFO manager: starting shutdown reason="context cancelled"
INFO manager/httpserver: shutting down
INFO manager/schedule/main.main.func2: stopped
INFO manager/httpserver: stopped
INFO manager/httpserver: stopped
INFO manager/StupidJobQueue: stopped
INFO manager: shutdown complete
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
| `Schedule(r, specs...)` | Run on a schedule: intervals, hourly, daily, or custom |
| `Recover(r)` | Catch panics and return them as errors |
| `Signal(r, signals...)` | Cancel context on OS signals |
| `Closer(c)` | Call `Close()` on context cancellation |
| `Func(fn)` | Adapt a `func(context.Context) error` to `Runnable` |
| `Named(r, name)` | Give a runnable a name, readable with `NameFromContext` |

## License

The MIT License (MIT)
