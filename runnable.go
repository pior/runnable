package runnable

import (
	"context"
)

// Runnable is the contract for anything that runs with a Go context, respects the cancellation contract,
// and expects the caller to handle errors.
type Runnable interface {
	Run(context.Context) error
}
