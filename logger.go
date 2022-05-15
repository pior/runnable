package runnable

import (
	stdlog "log"
	"os"
)

var log Logger = stdlog.New(os.Stdout, "[RUNNABLE] ", stdlog.Ldate|stdlog.Ltime)

// SetLogger replaces the default logger with a runnable.Logger.
func SetLogger(l Logger) {
	if l == nil {
		panic("Runnable: logger cannot be nil")
	}
	log = l
}

type Logger interface {
	Printf(format string, args ...interface{})
}
