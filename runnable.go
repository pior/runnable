package runnable

import (
	"context"
)

// Runnable is the contract for anything that runs with a Go context and expects
// the caller to handle errors.
//
// Cancellation contract: when the context is cancelled, Run must stop and return
// either nil or ctx.Err(), which is [context.Canceled] under [Manager] and [Run].
// Either is fine, both are a clean stop. Any other error, including
// [context.DeadlineExceeded], is a failure.
type Runnable interface {
	Run(context.Context) error
}
