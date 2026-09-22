# Runnable

[![Build Status](https://github.com/pior/runnable/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/pior/runnable/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/pior/runnable.svg)](https://pkg.go.dev/github.com/pior/runnable)
[![Go Report Card](https://goreportcard.com/badge/github.com/pior/runnable)](https://goreportcard.com/report/github.com/pior/runnable)

A zero-dependency Go library for orchestrating long-running processes with clean shutdown. Everything builds on a single interface:

```go
type Runnable interface {
    Run(context.Context) error
}
```

Shutdown is driven by context cancellation. When the context is cancelled, a runnable stops and returns either `nil` or `ctx.Err()`, both are a clean stop. Any other error, including `context.DeadlineExceeded`, is a failure.

## Manager

The `Manager` orchestrates multiple runnables with ordered shutdown. Runnables are organized in two tiers:

- **Processes** — the primary work (HTTP servers, workers, scheduled tasks)
- **Services** — infrastructure that processes depend on (databases, queues, metrics)

When shutdown is triggered (context cancelled or any runnable completes), processes are stopped first, then services. This ensures services remain available while processes drain.

```go
func main() {
    m := runnable.NewManager()
    m.RegisterService(jobQueue)
    m.RegisterProcess(runnable.NewHTTPServer(server))
    m.RegisterProcess(monitor)

    runnable.Run(m)
}
```

A `Manager` is itself a `Runnable`, so managers can be nested for independent shutdown ordering.

### Shutdown budget

`ShutdownTimeout` (default 30s) is the total budget for both shutdown phases. Processes get half of it, services get the rest: at least half, more when processes stop early. Runnables still running when their phase ends are reported with `ErrShutdownTimeout`.

It maps to a platform grace period such as Kubernetes `terminationGracePeriodSeconds`, which must exceed it to leave room for the process to exit. For example, with a 30s grace period:

```go
m := runnable.NewManager().ShutdownTimeout(25 * time.Second)
```

For nested managers, the inner budget must be smaller than the outer one. `NewHTTPServer` drains for 5s by default, which must stay below the process phase of the budget.

### Names

Each runnable in a manager runs with its full name in the context, such as `manager/restart/JobQueue`, used in log lines and errors. Read it with `NameFromContext(ctx)`, and set it with `Named(r, "api")`.

<details>
  <summary>Example logs</summary>

```
$ go run ./examples/example/
INFO manager/StupidJobQueue: started
INFO manager/httpserver: started
INFO manager/schedule/main.main.func2: started
INFO manager/httpserver: listening addr=localhost:8000
Task executed: 0
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
| `NewHTTPServer(server)` | Start and gracefully shut down a `*http.Server` |
| `NewRestart(r)` | Auto-restart on exit and on failure, with configurable limits and backoff |
| `NewSchedule(r, specs...)` | Run on a schedule: intervals, hourly, daily, cron, or custom |
| `Recover(r)` | Catch panics and return them as errors |
| `Signal(r, signals...)` | Cancel context on OS signals |
| `Closer(c)` | Call `Close()` on context cancellation, also `CloserErr`, `CloserCtx`, `CloserCtxErr` |
| `Func(fn)` | Adapt a `func(context.Context) error` to `Runnable` |
| `Named(r, name)` | Give a runnable a name, readable with `NameFromContext` |

## License

The MIT License (MIT)
