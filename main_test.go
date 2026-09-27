package runnable_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/pior/runnable"
)

// TestMain logs to stdout without timestamps, so that examples check their log
// lines in their Output. An example that silences the logs restores them with
// runnable.SetLogger(nil), which resets to [slog.Default].
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(stdout{}, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))
	runnable.SetLogger(nil)

	m.Run()
}

// stdout writes to the current [os.Stdout], which each example replaces to
// capture its output.
type stdout struct{}

func (stdout) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
