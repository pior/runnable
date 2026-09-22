# Changelog

## v1.1.0 (unreleased)

This release is breaking, under a minor version. The module path stays
`github.com/pior/runnable`.

### Breaking changes

| Before | After |
|---|---|
| `Manager()` returns unexported type | `NewManager() *Manager` |
| `Register` | `RegisterProcess` |
| `Register`/`RegisterService` return the registry | return nothing |
| Manager error is one flattened string | `errors.Join` chain, `%w` wrapped per runnable |
| `"X is still running"` text | `ErrShutdownTimeout` sentinel |
| `ShutdownTimeout` per phase | total for both phases, half for processes |
| `PanicError{value}` unexported | `PanicError{Value, Stack}`, `%+v` prints stack |
| HTTPServer default drain 30s | 5s |
| Manager log lines and error messages | new format, names from context |

### Added

- `Named(r, name)`: give a runnable a name.
- `NameFromContext(ctx)`: read the full name assigned by parents, such as `manager/restart/JobQueue`.
- `ErrShutdownTimeout`: sentinel for runnables still running when the shutdown budget expires.
- `HTTPServer(s).Listener(ln)`: serve on a provided `net.Listener`.
- `Cron(s)`: schedule spec for any type with a `Next(time.Time) time.Time` method, such as robfig/cron schedules.

### Fixed

- Manager no longer panics at `Run` on non-comparable runnables.
- Manager shutdown reason is `completed` when the runnable returned nil, `died` otherwise.
