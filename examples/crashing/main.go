package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/pior/runnable"
)

type crashing struct {
}

func (s *crashing) Run(ctx context.Context) error {
	fmt.Println("running... and I'm dead!")
	return errors.New("oops")
}

func main() {
	runnable.Run(runnable.NewRestart(&crashing{}).ErrorLimit(5))
}
