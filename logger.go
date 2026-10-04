package runnable

import (
	"errors"
	"log/slog"
)

var logger *slog.Logger

func init() {
	SetLogger(nil)
}

// SetLogger replaces the default logger with a [*slog.Logger].
// Passing nil resets to [slog.Default].
func SetLogger(l *slog.Logger) {
	if l == nil {
		l = slog.Default()
	}
	logger = l
}

// errorAttrs returns the log attributes of err. slog's TextHandler formats
// errors with %+v, which for a PanicError includes the stack: pass the message
// as a string to log the stack once.
func errorAttrs(err error) []any {
	if pe, ok := errors.AsType[*PanicError](err); ok {
		return []any{"error", err.Error(), "stack", string(pe.Stack)}
	}
	return []any{"error", err}
}
