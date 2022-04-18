package main

import (
	"context"
	"fmt"

	"github.com/pior/runnable"
	"github.com/sirupsen/logrus"
)

// Output:
// INFO[0000] manager: func(runnable.RunnableFunc) started  component=runnable
// Hello World!
// INFO[0000] manager: starting shutdown (func(runnable.RunnableFunc) died)  component=runnable
// INFO[0000] manager: func(runnable.RunnableFunc) cancelled  component=runnable
// INFO[0000] manager: func(runnable.RunnableFunc) stopped  component=runnable
// INFO[0000] manager: shutdown complete                    component=runnable

func main() {
	runnable.SetLogger(logrus.WithField("component", "runnable"))

	task := runnable.Func(func(ctx context.Context) error {
		fmt.Println("Hello World!")
		return nil
	})

	m := runnable.Manager(nil)
	m.Add(task)
	runnable.Run(m.Build())
}
