package runnable

import (
	stdlog "log"
	"os"
)

var log Logger

func init() {
	SetLogger(nil)
}

// SetLogger replaces the default logger with a runnable.Logger.
func SetLogger(l Logger) {
	if l == nil {
		l = stdlog.New(os.Stdout, "[RUNNABLE] ", stdlog.Ldate|stdlog.Ltime)
	}
	log = l
}

type Logger interface {
	Printf(format string, args ...any)
}

// Log logs a formatted message, prefixed by the runnable chain.
func Log(self any, format string, args ...any) {
	log.Printf(findName(self)+": "+format, args...)
}
