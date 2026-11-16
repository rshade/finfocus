package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/weblog"
)

// TestPreviewFailureKeepsPulumiOutputOutOfWebLogs (#2093): the warn line for a
// failed on-demand preview reaches the web Logs view, which every admitted
// user can read, so it names only the failure; Pulumi's output stays at debug.
func TestPreviewFailureKeepsPulumiOutputOutOfWebLogs(t *testing.T) {
	t.Parallel()
	const sentinel = "SENTINEL-TOKEN"
	for _, level := range []string{"info", "debug"} {
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			hub := weblog.NewHub(0)
			var terminal bytes.Buffer
			logger := logging.NewLoggerWithWriter(
				logging.Config{Level: level, Format: "json", Tee: hub.Tee()},
				&terminal,
			)
			session := NewSession(logger.WithContext(context.Background()), SessionOptions{
				Preview: func(context.Context) ([]engine.OverviewRow, error) {
					return nil, fmt.Errorf("web preview: %w", pulumi.PreviewError("error: "+sentinel))
				},
			})
			session.SetStateOnly(true)
			defer session.Close()
			_, events, unsubscribe := session.subscribe()
			defer unsubscribe()
			require.NoError(t, session.startPreview())
			for event := range events {
				if event.name == "error" {
					break
				}
			}

			warn, detail := previewLogEntries(t, hub)
			require.NotNil(t, warn, "the failure must still be logged at warn")
			assert.Equal(t, "pulumi preview failed", warn.Fields["error"])
			assert.NotContains(t, mustJSON(t, *warn), sentinel)
			if level == "info" {
				assert.Nil(t, detail)
				assert.NotContains(t, terminal.String(), sentinel)
				return
			}
			require.NotNil(t, detail, "debug must keep Pulumi's reason (#2087)")
			assert.Contains(t, detail.Fields["error"], sentinel)
		})
	}
}

func previewLogEntries(t *testing.T, hub *weblog.Hub) (*weblog.Entry, *weblog.Entry) {
	t.Helper()
	snapshot, _, cancel := hub.Subscribe(0)
	cancel()
	var warn, detail *weblog.Entry
	for i := range snapshot.Entries {
		entry := snapshot.Entries[i]
		switch entry.Message {
		case "pulumi preview failed":
			warn = &entry
		case "pulumi preview failure detail":
			detail = &entry
		}
	}
	return warn, detail
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
