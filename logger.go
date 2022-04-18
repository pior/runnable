package runnable

import (
	stdlog "log"
	"os"
)

var log Logger = &stdLogger{stdlog.New(os.Stdout, "[RUNNABLE] ", stdlog.Ldate|stdlog.Ltime)}

// SetLogger replaces the default logger with a runnable.Logger.
func SetLogger(logger Logger) {
	if logger == nil {
		panic("Runnable: logger cannot be nil")
	}
	log = logger
}

// SetStandardLogger replaces the default logger with a standard log instance.
func SetStandardLogger(logger StandardLogger) {
	SetLogger(&stdLogger{logger})
}

type Logger interface {
	// Infof logs with an info level.
	Infof(format string, args ...interface{})

	// Debugf logs with a debug level.
	Debugf(format string, args ...interface{})
}

type StandardLogger interface {
	Printf(format string, args ...interface{})
}

type stdLogger struct {
	logger StandardLogger
}

func (l *stdLogger) Infof(format string, args ...interface{}) {
	l.logger.Printf("INFO "+format, args...)
}

func (l *stdLogger) Debugf(format string, args ...interface{}) {
	l.logger.Printf("DBUG "+format, args...)
}
