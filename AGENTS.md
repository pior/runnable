# Runnable

Go library for managing the lifecycle of long-running processes.

## Goals

- **One abstraction.** Everything is a `Runnable`: `Run(context.Context) error`. Wrappers compose behavior around it; new features are wrappers, not new interfaces.
- **Small, visible API.** Everything configurable must show on pkg.go.dev: go/doc hides unexported types and their methods.
- **Built for main().** `Run`, `RunFunc`, `RunGroup` are main helpers: they handle signals and call `log.Fatal`. Library code must never exit.

## Invariants

- **Zero production dependencies.** Non-test code imports the standard library only. Never add a module that library code imports; test-only modules (testify) are fine. An integration like robfig/cron goes through a small interface (`Cron`), not an import.
- **Cancellation contract:** on cancellation, a runnable returns `nil` or `ctx.Err()`, both a clean stop. Any other error, including `context.DeadlineExceeded`, is a failure.
- **Shutdown is context cancellation.** No Stop/Close methods on runnables.
- **Manager ordering:** processes stop first, services after all processes stopped, so services stay available while processes drain. Each phase has its own timeout; the worst case shutdown is their sum.
- **Naming flows through the context.** Only `Manager` (sets `prefix/child`) and `Named` assign a name; other wrappers read it for their logs and pass the context through. Wrappers compose a fallback name at construction (e.g. `restart/JobQueue`). No `.Name()` setters, no exported naming interfaces.
- **Errors stay inspectable.** Manager errors are an `errors.Join` chain, each wrapped with `%w` and the runnable name. Never flatten errors to strings.

## API design rules

- **Configuration is functional options.** Option types are interfaces with an unexported method (`RestartOption{ applyRestart }`), so one option can serve several wrappers. No fluent setters.
- **Option names: share the meaning, or prefix the name.** A name is shared when it means the same thing on every wrapper that takes it (`DrainTimeout`, `Listener`): a later wrapper reuses it by returning a type that satisfies both option interfaces. A name is prefixed when its meaning belongs to one wrapper (`RestartLimit`, `RestartDelay`), so it never blocks another wrapper from using the plain word differently.
- **Argument order:** a function or runnable literal goes last when no variadic argument must (`Named(name, r)`, like `t.Run(name, fn)`). Variadic options and specs always go last (`Restart(r, opts...)`).
- **Constructors return `Runnable`** unless the returned type has exported methods callers need (`*Manager`).

## Conventions

- **Interface compliance:** use `var _ Interface = (*type)(nil)` compile-time checks, not test assertions.
- **README:** keep the wrappers table concise, describe what it does, not configuration details.
- **Tests:** use `127.0.0.1`, not `localhost`, to avoid IPv4/IPv6 resolution flakiness.
- **Logging:** prefix log messages with the full composed name (`logger.Info(name + ": started")`). No `"runnable"` structured field. Log lifecycle events in pairs (`"shutting down"` then `"stopped"`).

## Development

Uses DevBuddy (`bud`), see `dev.yml`.
