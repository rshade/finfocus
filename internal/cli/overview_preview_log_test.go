package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/stackload"
	"github.com/rshade/finfocus/internal/weblog"
)

type failingPreviewRunner struct{ stderr string }

func (r failingPreviewRunner) Run(context.Context, string, string, []string, ...string) ([]byte, []byte, error) {
	return nil, []byte(r.stderr), errors.New("exit status 1")
}

// TestStartupPreviewFallbackKeepsPulumiOutputOutOfWebLogs (#2093): the startup
// fallback follows the on-demand preview's rule. Its warn line reaches the web
// Logs view, so it carries only the safe reason, and Pulumi's output stays at
// debug.
//
//nolint:paralleltest // SetRunnerForTest replaces the process-wide Pulumi runner.
func TestStartupPreviewFallbackKeepsPulumiOutputOutOfWebLogs(t *testing.T) {
	const sentinel = "SENTINEL-TOKEN"
	t.Cleanup(pulumi.SetRunnerForTest(failingPreviewRunner{stderr: "error: " + sentinel}))
	for _, level := range []string{"info", "debug"} {
		t.Run(level, func(t *testing.T) {
			hub := weblog.NewHub(0)
			var terminal bytes.Buffer
			logger := logging.NewLoggerWithWriter(
				logging.Config{Level: level, Format: "json", Tee: hub.Tee()},
				&terminal,
			)
			base := &stackload.Snapshot{ProjectDir: t.TempDir(), StackName: "dev"}

			got, err := loadTUIOverviewPreview(logger.WithContext(context.Background()), &OverviewPipeline{}, base, nil)
			require.NoError(t, err)
			require.Same(t, base, got, "a failed preview falls back to current state")

			snapshot, _, cancel := hub.Subscribe(0)
			cancel()
			var warned, detailed bool
			for _, entry := range snapshot.Entries {
				errorText, _ := entry.Fields["error"].(string)
				switch entry.Message {
				case "pulumi preview failed; showing current state":
					warned = true
					assert.Equal(t, "loading Pulumi plan: read_failed", errorText)
				case "pulumi preview failure detail":
					detailed = true
					assert.Contains(t, errorText, sentinel, "debug must keep Pulumi's reason (#2087)")
				default:
					assert.NotContains(t, errorText, sentinel)
				}
			}
			assert.True(t, warned, "the fallback must still be logged at warn")
			assert.Equal(t, level == "debug", detailed)
			assert.Equal(t, level == "debug", strings.Contains(terminal.String(), sentinel))
		})
	}
}
