package plugin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// NOTE: The fallback logic is implemented in the CLI layer (internal/cli/plugin_install.go),
// not in the registry.Installer. These tests verify the underlying registry behavior that
// enables the CLI fallback feature:
// - Installer returns "no asset found" errors for versions without platform assets
// - FallbackInfo struct correctly identifies when fallback occurred
// - FindReleaseWithFallbackInfo finds fallback versions when requested version lacks assets

// FallbackMockConfig extends MockRegistryConfig with fallback testing capabilities.
type FallbackMockConfig struct {
	// Plugins maps plugin name to available versions (latest version first)
	Plugins map[string][]string
	// VersionsWithoutAssets lists versions that exist but have no platform-compatible assets
	VersionsWithoutAssets map[string][]string
}

// StartMockRegistryWithFallback creates a mock registry server that supports fallback scenarios.
// It can return releases without assets for specific versions, enabling fallback testing.
func StartMockRegistryWithFallback(t *testing.T, cfg FallbackMockConfig) (*httptest.Server, func()) {
	t.Helper()

	server := httptest.NewServer(&fallbackMockRegistry{t: t, cfg: cfg, store: newMockArtifactStore()})
	return server, server.Close
}

// fallbackMockRegistry serves GitHub-style release responses where configured
// versions return no platform assets.
type fallbackMockRegistry struct {
	t     *testing.T
	cfg   FallbackMockConfig
	store *mockArtifactStore
}

func (m *fallbackMockRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/download/") {
		m.store.serveDownload(w, r)
		return
	}

	pluginName, versions, ok := parsePluginRequest(m.cfg.Plugins, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	serverURL := "http://" + r.Host
	switch {
	case strings.HasSuffix(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/tags/"):
		// List all releases for fallback search
		releases := make([]MockRelease, 0, len(versions))
		for _, v := range versions {
			releases = append(releases, m.releaseFor(pluginName, v, serverURL))
		}
		_ = json.NewEncoder(w).Encode(releases)
	case strings.Contains(r.URL.Path, "/releases/latest"):
		_ = json.NewEncoder(w).Encode(m.releaseFor(pluginName, versions[0], serverURL))
	case strings.Contains(r.URL.Path, "/releases/tags/"):
		tag := releaseTag(r.URL.Path)
		if !slices.Contains(versions, tag) {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(m.releaseFor(pluginName, tag, serverURL))
	default:
		http.NotFound(w, r)
	}
}

// releaseFor creates a release response; versions configured in
// VersionsWithoutAssets return a release with no platform assets.
func (m *fallbackMockRegistry) releaseFor(pluginName, tagName, serverURL string) MockRelease {
	if slices.Contains(m.cfg.VersionsWithoutAssets[pluginName], tagName) {
		return MockRelease{
			TagName: tagName,
			Assets:  []MockAsset{}, // No assets for this platform
		}
	}
	return newMockRelease(m.t, m.store, pluginName, tagName, serverURL)
}

// TestPluginInstall_VersionWithoutAssets_ReturnsError tests that installer returns
// "no asset found" error when the requested version lacks platform assets [T020].
// This error is what triggers the CLI fallback logic.
func TestPluginInstall_VersionWithoutAssets_ReturnsError(t *testing.T) {
	// Setup mock with v1.1.0 having no assets, v1.0.0 having assets
	cfg := FallbackMockConfig{
		Plugins: map[string][]string{
			"fallback": {"v1.1.0", "v1.0.0"}, // v1.1.0 is latest
		},
		VersionsWithoutAssets: map[string][]string{
			"fallback": {"v1.1.0"}, // v1.1.0 has no platform assets
		},
	}
	server, cleanup := StartMockRegistryWithFallback(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Try to install v1.1.0 which has no assets
	specifier := "github.com/example/finfocus-plugin-fallback@v1.1.0"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	// Installer should return error - the CLI layer handles fallback
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no asset found")
}

// TestPluginInstall_NoFallback tests the --no-fallback flag behavior [T027].
// When --no-fallback is set and the requested version lacks assets,
// installation should fail immediately without attempting fallback.
func TestPluginInstall_NoFallback(t *testing.T) {
	// Setup mock with v1.1.0 having no assets
	cfg := FallbackMockConfig{
		Plugins: map[string][]string{
			"strict": {"v1.1.0", "v1.0.0"},
		},
		VersionsWithoutAssets: map[string][]string{
			"strict": {"v1.1.0"},
		},
	}
	server, cleanup := StartMockRegistryWithFallback(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:     true,
		PluginDir:  pluginDir,
		NoFallback: true, // Disable fallback
	}

	// Try to install v1.1.0 which has no assets
	specifier := "github.com/example/finfocus-plugin-strict@v1.1.0"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	// Should fail with "no asset found" error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no asset found")
}

// TestPluginInstall_FallbackDeclinedNonTTY tests non-interactive fallback behavior [T013].
// In non-TTY mode without --fallback-to-latest, installation should fail
// when the requested version lacks assets.
func TestPluginInstall_FallbackDeclinedNonTTY(t *testing.T) {
	// Setup mock with v1.1.0 having no assets
	cfg := FallbackMockConfig{
		Plugins: map[string][]string{
			"nontty": {"v1.1.0", "v1.0.0"},
		},
		VersionsWithoutAssets: map[string][]string{
			"nontty": {"v1.1.0"},
		},
	}
	server, cleanup := StartMockRegistryWithFallback(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	// No FallbackToLatest, no NoFallback - default behavior in non-TTY
	opts := registry.InstallOptions{
		NoSave:    true,
		PluginDir: pluginDir,
	}

	// Try to install v1.1.0 which has no assets
	specifier := "github.com/example/finfocus-plugin-nontty@v1.1.0"
	_, err := installer.Install(context.Background(), specifier, opts, nil)

	// Should fail because we're in non-TTY and can't prompt
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no asset found")
}

// TestFindReleaseWithFallbackInfo_FindsFallbackVersion tests that FindReleaseWithFallbackInfo
// correctly identifies a fallback version when the requested version lacks assets [T020].
// This is the core registry method that the CLI uses for fallback logic.
func TestFindReleaseWithFallbackInfo_FindsFallbackVersion(t *testing.T) {
	// Setup: v1.2.0 and v1.1.0 have no assets, only v1.0.0 has assets
	cfg := FallbackMockConfig{
		Plugins: map[string][]string{
			"multi": {"v1.2.0", "v1.1.0", "v1.0.0"},
		},
		VersionsWithoutAssets: map[string][]string{
			"multi": {"v1.2.0", "v1.1.0"}, // Both latest versions lack assets
		},
	}
	server, cleanup := StartMockRegistryWithFallback(t, cfg)
	defer cleanup()

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL

	// Use FindReleaseWithFallbackInfo to find a version with assets
	info, err := client.FindReleaseWithFallbackInfo(
		context.Background(), "example", "finfocus-plugin-multi", "v1.2.0", "multi", nil,
	)

	// Should find v1.0.0 as fallback
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.True(t, info.WasFallback)
	assert.Equal(t, "v1.2.0", info.RequestedVersion)
	assert.Equal(t, "v1.0.0", info.Release.TagName)
	assert.Contains(t, info.FallbackReason, "no compatible assets")
}

// TestPluginInstall_NoFallbackNeeded tests when requested version has assets [T020].
// When the requested version has compatible assets, no fallback should occur.
func TestPluginInstall_NoFallbackNeeded(t *testing.T) {
	cfg := FallbackMockConfig{
		Plugins: map[string][]string{
			"normal": {"v1.1.0", "v1.0.0"},
		},
		// No versions without assets - all versions work
	}
	server, cleanup := StartMockRegistryWithFallback(t, cfg)
	defer cleanup()

	pluginDir := setupTestPluginDir(t)
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	client := registry.NewGitHubClient()
	client.BaseURL = server.URL
	installer := registry.NewInstallerWithClient(client, pluginDir)

	opts := registry.InstallOptions{
		NoSave:           true,
		PluginDir:        pluginDir,
		FallbackToLatest: true, // Flag set but shouldn't matter
	}

	// Install v1.1.0 which has assets
	specifier := "github.com/example/finfocus-plugin-normal@v1.1.0"
	result, err := installer.Install(context.Background(), specifier, opts, nil)

	// Should succeed without fallback
	require.NoError(t, err)
	assert.Equal(t, "normal", result.Name)
	assert.Equal(t, "v1.1.0", result.Version)
	assert.False(t, result.WasFallback)
	assert.Empty(t, result.RequestedVersion)
}
