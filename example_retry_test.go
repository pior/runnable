package runnable_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pior/runnable"
)

func ExampleRetry() {
	// A migration that fails until the database is up.
	runs := 0
	migrate := runnable.Named("migrate", runnable.Func(func(context.Context) error {
		runs++
		if runs < 3 {
			return errors.New("connection refused")
		}
		fmt.Println("migrated")
		return nil
	}))

	r := runnable.Retry(migrate,
		runnable.ErrorLimit(5),
		runnable.ErrorBackoff(func(int) time.Duration { return 10 * time.Millisecond }),
	)

	fmt.Println(r.Run(context.Background()))

	// Output:
	// level=INFO msg="retry/migrate: failed, retrying" error="connection refused" errors=1 delay=10ms
	// level=INFO msg="retry/migrate: failed, retrying" error="connection refused" errors=2 delay=10ms
	// migrated
	// <nil>
}

func ExampleExponentialBackoff() {
	backoff := runnable.ExponentialBackoff(time.Second, 10*time.Second)

	for errors := range 5 {
		fmt.Println(backoff(errors + 1))
	}

	// Output:
	// 1s
	// 2s
	// 4s
	// 8s
	// 10s
}
