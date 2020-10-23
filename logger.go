package runnable

import (
	"fmt"
	"log"
)

var logInfo = func(msg string) {
	log.Printf("[RUNNABLE] %s", msg)
}

// SetLogger changes the logger used by this library. The default is log.Printf.
//
// Example for github.com/rs/zerolog:
//
//	runnable.SetLogger(func(msg string) {
// 		log.Info().Str("component", "runnable").Msg(msg)
//	})
//
// Example for github.com/rs/zerolog:
//
//	runnable.SetLogger(func(msg string) {
// 		logrus.WithField("component", "runnable").Info(msg)
//	})
//
func SetLogger(logger func(msg string)) {
	logInfo = logger
}

func logInfof(format string, args ...interface{}) {
	logInfo(fmt.Sprintf(format, args...))
}
