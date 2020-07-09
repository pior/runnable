package main

import (
	"context"
	"fmt"

	"github.com/pior/runnable"
)

func main() {
	runnable.RunFunc(run)
}

func run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Println("y")
	}
}
