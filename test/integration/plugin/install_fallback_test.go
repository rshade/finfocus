package plugin_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
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

	var mu sync.Mutex
	artifacts := make(map[string][]byte)

	// Helper to check if a version should have no assets
	hasNoAssets := func(pluginName, version string) bool {
		noAssetVersions, ok := cfg.VersionsWithoutAssets[pluginName]
		if !ok {
			return false
		}
		for _, v := range noAssetVersions {
			if v == version {
				return true
			}
		}
		return false
	}

	// Helper to create a release response
	createReleaseForPlugin := func(pluginName, tagName, serverURL string) MockRelease {
		// If this version should have no assets, return empty assets
		if hasNoAssets(pluginName, tagName) {
			return MockRelease{
				TagName: tagName,
				Assets:  []MockAsset{}, // No assets for this platform
			}
		}

		osName := runtime.GOOS
		arch := runtime.GOARCH
		ext := "tar.gz"
		if osName == "windows" {
			ext = "zip"
		}

		assetName := fmt.Sprintf("%s_%s_%s_%s.%s", pluginName, tagName, osName, arch, ext)
		downloadPath := fmt.Sprintf("/download/%s", assetName)

		mu.Lock()
		if _, exists := artifacts[assetName]; !exists {
			content := CreateTestPluginArchive(t, pluginName, tagName, osName, arch)
			artifacts[assetName] = content
		}
		artifactSize := int64(len(artifacts[assetName]))
		mu.Unlock()

		return MockRelease{
			TagName: tagName,
			Assets: []MockAsset{
				{
					Name:               assetName,
					BrowserDownloadURL: serverURL + downloadPath,
					Size:               artifactSize,
				},
			},
		}
	}

	// Helper to create list of releases for fallback search
	createReleasesList := func(pluginName, serverURL string) []MockRelease {
		versions, exists := cfg.Plugins[pluginName]
		if !exists {
			return nil
		}
		releases := make([]MockRelease, 0, len(versions))
		for _, v := range versions {
			releases = append(releases, createReleaseForPlugin(pluginName, v, serverURL))
		}
		return releases
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle download requests
		if strings.HasPrefix(r.URL.Path, "/download/") {
			filename := strings.TrimPrefix(r.URL.Path, "/download/")
			mu.Lock()
			content, ok := artifacts[filename]
			mu.Unlock()
			if ok {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(content)
				return
			}
			http.NotFound(w, r)
			return
		}

		// Parse owner/repo from path
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.NotFound(w, r)
			return
		}

		repo := parts[2]
		pluginName := strings.TrimPrefix(repo, "finfocus-plugin-")

		versions, exists := cfg.Plugins[pluginName]
		if !exists || len(versions) == 0 {
			http.NotFound(w, r)
			return
		}

		// Handle /releases endpoint (list all releases for fallback search)
		if strings.HasSuffix(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/tags/") {
			releases := createReleasesList(pluginName, "http://"+r.Host)
			_ = json.NewEncoder(w).Encode(releases)
			return
		}

		// Handle /releases/latest
		if strings.Contains(r.URL.Path, "/releases/latest") {
			release := createReleaseForPlugin(pluginName, versions[0], "http://"+r.Host)
			_ = json.NewEncoder(w).Encode(release)
			return
		}

		// Handle /releases/tags/{tag}
		if strings.Contains(r.URL.Path, "/releases/tags/") {
			tag := parts[len(parts)-1]
			found := false
			for _, v := range versions {
				if v == tag {
					found = true
					break
				}
			}
			if !found {
				http.NotFound(w, r)
				return
			}
			release := createReleaseForPlugin(pluginName, tag, "http://"+r.Host)
			_ = json.NewEncoder(w).Encode(release)
			return
		}

		http.NotFound(w, r)
	}))

	return server, server.Close
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
	_, err := installer.Install(specifier, opts, nil)

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
	_, err := installer.Install(specifier, opts, nil)

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
	_, err := installer.Install(specifier, opts, nil)

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
	info, err := client.FindReleaseWithFallbackInfo("example", "finfocus-plugin-multi", "v1.2.0", "multi", nil)

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
	result, err := installer.Install(specifier, opts, nil)

	// Should succeed without fallback
	require.NoError(t, err)
	assert.Equal(t, "normal", result.Name)
	assert.Equal(t, "v1.1.0", result.Version)
	assert.False(t, result.WasFallback)
	assert.Empty(t, result.RequestedVersion)
}
