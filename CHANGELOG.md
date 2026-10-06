# Changelog

## Unreleased

### Added

- `ExponentialBackoff(base, maxDelay)`: a backoff for `ErrorBackoff` that doubles the delay with each consecutive error, up to `maxDelay`.

### Changed

- `OnError` returns an `ErrorOption`, shared by `Restart` and `Retry`. `Restart` calls it with each error, including the last one returned at the error limit.

## v1.1.1 (2026-10-06)

### Added

- `OnError(fn)`: `Retry` calls `fn` with each error, including the last one returned at the error limit.

## v1.1.0 (2026-10-04)

This release is breaking, under a minor version. The module path stays
`github.com/pior/runnable`.

### Breaking changes

| Before | After |
|---|---|
| Go 1.25 | Go 1.26 minimum |
| `Manager()` returns unexported type | `NewManager() *Manager` |
| `Register` | `RegisterProcess` |
| `Register`/`RegisterService` return the registry | return nothing |
| `ManagerRegistry` interface | removed, use `*Manager` |
| Manager error is one flattened string | `errors.Join` chain, `%w` wrapped per runnable |
| `"X is still running"` text | `ErrShutdownTimeout` sentinel |
| `Manager().ShutdownTimeout(d)`, 10s per phase | `NewManager(ProcessShutdownTimeout(d), ServiceShutdownTimeout(d))`, 15s and 10s |
| `PanicError{value}` unexported | `PanicError{Value, Stack}`, `%+v` prints stack |
| HTTPServer default drain 30s | 5s |
| Manager log lines and error messages | new format, names from context |
| HTTPServer logs `shutting down`, `stopped`, `stopped with error` | `draining`, then `drained`, `drain timed out` or `drain failed`; errors are returned, not logged |
| `Run`, `RunFunc`, `RunGroup` exit with `log.Fatal` | log `stopped with error` with the `SetLogger` logger, exit with status 1 |
| `.Name(string)` on `Func`, `HTTPServer`, `Schedule`, `Manager` | removed, use `Named(name, r)` |
| `HTTPServer(s).ShutdownTimeout(d).Listener(ln)` | `HTTPServer(s, DrainTimeout(d), Listener(ln))` |
| `Restart(r).Limit(n).ErrorLimit(n).Delay(d).ErrorBackoff(fn).ErrorResetAfter(d)` | `Restart(r, RestartLimit(n), ErrorLimit(n), RestartDelay(d), ErrorBackoff(fn), ErrorResetAfter(d))` |
| `Schedule(r, specs...)`, runs at whichever spec fires next | `Schedule(r, spec, opts...)`, one spec: use several schedules, or `Cron` |
| `HTTPServer`, `Restart`, `Schedule`, `Func` return unexported types | return `Runnable` |
| `Closer` wrappers return `*RunnableError` | `RunnableError` removed, the `Close` error is wrapped with `%w` |

### Added

- `Named(name, r)`: give a runnable a name.
- `NameFromContext(ctx)`: read the full name assigned by parents, such as `manager/restart/JobQueue`.
- `ErrShutdownTimeout`: sentinel for runnables still running when the shutdown budget expires.
- `HTTPServer(s, Listener(ln))`: serve on a provided `net.Listener`.
- `ScheduleContinueOnError()`: `Schedule` logs a failed run and runs again at the next tick, instead of stopping.
- `ServerOption`: `DrainTimeout` and `Listener`, shared by the server wrappers.
- `Retry(r, opts...)`: run again after errors until success.
- `ErrorOption`: `ErrorLimit`, `ErrorBackoff` and `ErrorResetAfter`, shared by `Restart` and `Retry`.
- `Delay(d, r)`: wait for d before running r, to stagger the start of jobs.
- `Timeout(d, r)`: cancel a runnable running for longer than d.
- `Cron(s)`: schedule spec for any type with a `Next(time.Time) time.Time` method, such as robfig/cron schedules.

### Fixed

- `HTTPServer` logs `listening` once the socket is open, with the port picked by the system for `:0`. It logged before listening, even when listening failed.
- `Signal` stops listening when `Run` returns. It leaked a goroutine that kept catching the signals.
- `RestartLimit` counts only restarts after a successful run, as documented. Restarts after errors counted too.
- Manager no longer panics at `Run` on non-comparable runnables.
- Manager shutdown reason is `completed` when the runnable returned nil, `died` otherwise.
- `Restart` logs `failed, restarting` with the error before restarting. Errors and panics, with their stack, were dropped.
