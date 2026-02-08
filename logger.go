package runnable

import "log/slog"

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
