package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

// Configures process-wide environment and project selection.
func TestActualCloudModes(t *testing.T) {
	for _, tc := range []struct {
		name, mode, backend, token string
		code, exit                 int
		reason                     string
		disabled, locked           bool
	}{
		{name: "off", mode: "false", token: "t", code: 200},
		{name: "auto file", mode: "auto", backend: "file:///tmp/state", token: "t", code: 200},
		{name: "auto missing token", mode: "auto", code: 200},
		{name: "auto rejected", mode: "auto", token: "t", code: 401},
		{name: "required file", mode: "true", backend: "file:///tmp/state", token: "t", code: 200, exit: 2, reason: "Cloud history requires a Pulumi Cloud backend"},
		{name: "required insecure", mode: "true", backend: "http://remote.example", token: "t", code: 200, exit: 1, reason: "must use HTTPS"},
		{name: "required missing token", mode: "true", code: 200, exit: 1, reason: "not_logged_in"},
		{name: "required rejected", mode: "true", token: "t", code: 401, exit: 1, reason: "token_rejected"},
		{name: "required forbidden", mode: "true", token: "t", code: 403, exit: 1, reason: "no_permission"},
		{name: "required disabled", mode: "true", token: "t", code: 200, exit: 2, reason: "cost.history.enabled", disabled: true},
		{name: "required locked", mode: "true", token: "t", code: 200, exit: 1, reason: "locked", locked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupCloudModeProject(t, tc.mode)
			t.Setenv("PULUMI_HOME", t.TempDir())
			t.Setenv("PULUMI_ACCESS_TOKEN", tc.token)
			if tc.disabled {
				t.Setenv("FINFOCUS_HISTORY_ENABLED", "false")
			}
			if tc.locked {
				store, err := history.NewBoltStore(
					context.Background(),
					filepath.Join(config.ResolveConfigDir(), "history"),
					true,
					90,
				)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
			}
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"updates":[]}`))
			}))
			t.Cleanup(server.Close)
			backend := tc.backend
			if backend == "" {
				backend = server.URL
			}
			t.Setenv("PULUMI_BACKEND_URL", backend)
			now := time.Now().Truncate(time.Second)
			result := axtest.Run(
				context.Background(),
				t,
				NewRootCmd("test"),
				[]string{
					"cost",
					"actual",
					"--from",
					now.Add(-time.Hour).Format(time.RFC3339),
					"--to",
					now.Format(time.RFC3339),
					"--output",
					"json",
				},
			)
			assert.Equal(t, tc.exit, result.ExitCode, string(result.Stderr))
			if tc.reason != "" {
				assert.Contains(t, string(result.Stderr), tc.reason)
				assert.Empty(t, result.Stdout)
			}
			if tc.backend != "" || tc.mode == "false" || tc.disabled || tc.locked || tc.token == "" {
				assert.Zero(t, requests.Load())
			}
		})
	}
}
