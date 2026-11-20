package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/webui"
)

func TestReuseInstalledVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "demo", "v1.0.0"), 0o750))

	present := registry.InstallOptions{}
	reuseInstalledVersion(dir, "demo", "v1.0.0", &present)
	assert.True(t, present.Force)

	missing := registry.InstallOptions{}
	reuseInstalledVersion(dir, "demo", "v9.9.9", &missing)
	assert.False(t, missing.Force)

	latest := registry.InstallOptions{}
	reuseInstalledVersion(dir, "demo", "", &latest)
	assert.False(t, latest.Force)
}

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

func TestPluginPlan_SecureModePlansReleaseWithChecksums(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/rshade/finfocus-plugin-aws-public/releases/latest" {
			http.NotFound(w, r)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"assets": []map[string]any{
				{"name": "aws-public_v1.0.0_linux_amd64.tar.gz", "size": 1},
				{"name": "checksums.txt", "size": 1},
			},
		}))
	}))
	t.Cleanup(server.Close)
	t.Setenv("FINFOCUS_GITHUB_API", server.URL)
	t.Setenv("FINFOCUS_CATALOG_FILE", "")
	t.Setenv(config.EnvSecurePlugins, "true")

	plan, err := pluginPlan(context.Background(), "aws-public", webui.PluginPlanRequest{Action: "install"})

	require.NoError(t, err)
	assert.Equal(t, "present", plan.ChecksumFile)
}

func TestSetupRetryHint(t *testing.T) {
	t.Parallel()

	refused := fmt.Errorf("%w: %w", registry.ErrSecureModeRefusal, registry.ErrAssetNotInChecksums)
	download := fmt.Errorf("%w: %w", registry.ErrSecureModeRefusal, errors.New("download failed"))

	assert.Empty(t, setupRetryHint("aws-public", refused))
	assert.Contains(t, setupRetryHint("aws-public", download), "finfocus plugin install aws-public")
	assert.Contains(t, setupRetryHint("aws-public", errors.New("boom")), "finfocus plugin install aws-public")
}
