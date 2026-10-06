package runnable

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExponentialBackoff(t *testing.T) {
	// delays returns the delays for errors 1 to n.
	delays := func(backoff func(int) time.Duration, n int) string {
		var out []time.Duration
		for errors := range n {
			out = append(out, backoff(errors+1))
		}
		return fmt.Sprint(out)
	}

	t.Run("doubles up to the max delay", func(t *testing.T) {
		backoff := ExponentialBackoff(time.Second, time.Minute)
		require.Equal(t, "[1s 2s 4s 8s 16s 32s 1m0s 1m0s]", delays(backoff, 8))
	})

	t.Run("sub-second base", func(t *testing.T) {
		backoff := ExponentialBackoff(100*time.Millisecond, 10*time.Second)
		require.Equal(t, "[100ms 200ms 400ms 800ms 1.6s 3.2s 6.4s 10s]", delays(backoff, 8))
	})

	t.Run("max delay below base", func(t *testing.T) {
		backoff := ExponentialBackoff(time.Minute, time.Second)
		require.Equal(t, "[1s 1s]", delays(backoff, 2))
	})

	t.Run("no overflow on many errors", func(t *testing.T) {
		backoff := ExponentialBackoff(time.Second, time.Duration(1<<62))
		require.Equal(t, time.Duration(1<<62).String(), backoff(1000).String())
	})
}
