package runnable

import (
	"context"
	"fmt"
	"strings"
)

// Runnable is the contract for anything that runs with a Go context, respects the concellation contract,
// and expects the caller to handle errors.
type Runnable interface {
	Run(context.Context) error
}

// // Periodic returns a runnable that will periodically run the runnable passed in argument.
// func Periodic(period time.Duration, runnable Runnable) Runnable {
// 	return nil
// }

// // Timeout returns a runnable that will periodically run the runnable passed in argument.
// func Timeout(timeout time.Duration, runnable Runnable) Runnable {
// 	return nil
// }

// // RunAndExit runs the runnable and sets the exit code to 1 if the runnable returns an error.
// func RunAndExit(timeout time.Duration, runnable Runnable) {
// }

func nameOfRunnable(runnable Runnable) string {
	if r, ok := runnable.(interface{ name() string }); ok {
		return r.name()
	}
	return strings.TrimLeft(fmt.Sprintf("%T", runnable), "*")
}
