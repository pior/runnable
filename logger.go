package runnable

import (
	stdlog "log"
	"os"
)

var log Logger = &stdLogger{stdlog.New(os.Stdout, "[RUNNABLE] ", stdlog.Ldate|stdlog.Ltime)}

type Logger interface {
	// Infof logs with an info level.
	Infof(format string, args ...interface{})

	// Debugf logs with a debug level.
	Debugf(format string, args ...interface{})
}

type stdLogger struct {
	logger *stdlog.Logger
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("INFO "+format, args...)
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("DBUG "+format, args...)
}

// SetLogger replaces the default logger.
func SetLogger(l Logger) {
	log = l
}
