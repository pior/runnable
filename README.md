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
    m.RegisterProcess(runnable.HTTPServer(server))
    m.RegisterProcess(monitor)

    runnable.Run(m)
}
```

A `Manager` is itself a `Runnable`, so managers can be nested for independent shutdown ordering.

### Shutdown budget

Each shutdown phase has its own timeout: `ProcessShutdownTimeout` (default 15s) starts with the shutdown, `ServiceShutdownTimeout` (default 10s) starts when services are cancelled. Runnables still running when their phase ends are reported with `ErrShutdownTimeout`.

The worst case is the sum, 25s by default. Keep it below the platform grace period, such as Kubernetes `terminationGracePeriodSeconds` (default 30s), to leave room for the process to exit. For example, with a 60s grace period:

```go
m := runnable.NewManager(
    runnable.ProcessShutdownTimeout(45*time.Second),
    runnable.ServiceShutdownTimeout(10*time.Second),
)
```

For nested managers, the inner sum must stay below the timeout of the phase the inner manager runs in. `HTTPServer` drains for 5s by default, which must stay below `ProcessShutdownTimeout`.

### Names

Each runnable in a manager runs with its full name in the context, such as `manager/restart/JobQueue`, used in log lines and errors. Read it with `NameFromContext(ctx)`, and set it with `Named("api", r)`.

<details>
  <summary>Example logs</summary>

Output of the package example in [example_test.go](example_test.go): a job queue service, a scheduled cleanup task, and an app process whose completion shuts the manager down.

```
level=INFO msg="manager/JobQueue: started"
level=INFO msg="manager/schedule/CleanupTask: started"
level=INFO msg="manager/app: started"
JobQueue: cleanup-1
JobQueue: cleanup-2
JobQueue: cleanup-3
level=INFO msg="manager/app: stopped"
level=INFO msg="manager: starting shutdown" reason="app completed"
level=INFO msg="manager/schedule/CleanupTask: stopped"
level=INFO msg="manager/JobQueue: stopped"
level=INFO msg="manager: shutdown complete"
```

</details>

## Entrypoints

`Run`, `RunFunc`, and `RunGroup` are intended as `main()` helpers. They handle OS signals (SIGINT/SIGTERM) and on error, log it and exit with status 1.

```go
func main() {
    runnable.Run(myApp)
}
```

## Wrappers

Wrappers compose behavior around a `Runnable`:

| Wrapper | Description |
|---------|-------------|
| `HTTPServer(server, opts...)` | Start and gracefully shut down a `*http.Server` |
| `GRPCServer(addr, server, opts...)` | Start and gracefully stop a `*grpc.Server` |
| `Restart(r, opts...)` | Auto-restart on exit and on failure, with configurable limits and backoff |
| `Retry(r, opts...)` | Run again after errors until it succeeds, with configurable limit and backoff |
| `Schedule(r, spec, opts...)` | Run on a schedule: intervals, hourly, daily, cron, or custom |
| `Recover(r)` | Catch panics and return them as errors |
| `Timeout(d, r)` | Cancel a runnable that runs for too long |
| `Delay(d, r)` | Wait before starting a runnable |
| `Signal(r, signals...)` | Cancel context on OS signals |
| `Closer(c)` | Call `Close()` on context cancellation, also `CloserErr`, `CloserCtx`, `CloserCtxErr` |
| `Func(fn)` | Adapt a `func(context.Context) error` to `Runnable` |
| `Named(name, r)` | Give a runnable a name, readable with `NameFromContext` |

## License

The MIT License (MIT)
