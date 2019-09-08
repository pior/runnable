package runnable

import (
	stdlog "log"
	"os"
)

var log = &stdLogger{stdlog.New(os.Stdout, "[RUNNABLE] ", stdlog.Ldate|stdlog.Ltime)}

type Logger interface {
	// Warnf logs with a warning level.
	Warnf(format string, args ...interface{})

	// Infof logs with an info level.
	Infof(format string, args ...interface{})

	// Debugf logs with a debug level.
	Debugf(format string, args ...interface{})
}

type stdLogger struct {
	logger *stdlog.Logger
}

func (l *stdLogger) Warnf(format string, args ...interface{}) {
	l.logger.Printf("WARN "+format, args...)
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("INFO "+format, args...)
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("DBUG "+format, args...)
}
