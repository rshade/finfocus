package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/webui"
)

func TestRefuseMissingChecksumInSecureMode(t *testing.T) {
	t.Run("on refuses with a conflict the page can show", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "true")

		err := refuseMissingChecksumInSecureMode()

		var status *webui.StatusError
		require.ErrorAs(t, err, &status)
		assert.Equal(t, http.StatusConflict, status.Status)
		assert.Equal(t, "checksum_required", status.Code)
		assert.Contains(t, status.Text, config.EnvSecurePlugins)
	})

	t.Run("off leaves the confirmation to the user", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "false")

		require.NoError(t, refuseMissingChecksumInSecureMode())
	})

	t.Run("a bad value is an error, not a pass", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "maybe")

		require.Error(t, refuseMissingChecksumInSecureMode())
	})
}

func TestPluginPlan_SecureModeRefusesReleaseWithoutChecksums(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/rshade/finfocus-plugin-aws-public/releases/latest" {
			http.NotFound(w, r)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"assets":   []map[string]any{{"name": "aws-public_v1.0.0_linux_amd64.tar.gz", "size": 1}},
		}))
	}))
	t.Cleanup(server.Close)
	t.Setenv("FINFOCUS_GITHUB_API", server.URL)
	t.Setenv("FINFOCUS_CATALOG_FILE", "")
	request := webui.PluginPlanRequest{Action: "install"}

	t.Run("on answers 409 before any confirmation", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "true")

		_, err := pluginPlan(context.Background(), "aws-public", request)

		var status *webui.StatusError
		require.ErrorAs(t, err, &status)
		assert.Equal(t, http.StatusConflict, status.Status)
		assert.Equal(t, "checksum_required", status.Code)
	})

	t.Run("off still plans a missing checksum for confirmation", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "false")

		plan, err := pluginPlan(context.Background(), "aws-public", request)

		require.NoError(t, err)
		assert.Equal(t, "missing", plan.ChecksumFile)
	})
}
