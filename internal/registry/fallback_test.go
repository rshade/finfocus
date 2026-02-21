package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// TestFallbackInfo tests the FallbackInfo struct fields and state.
func TestFallbackInfo(t *testing.T) {
	t.Run("non-fallback scenario", func(t *testing.T) {
		info := registry.FallbackInfo{
			Release:          &registry.GitHubRelease{TagName: "v1.0.0"},
			Asset:            &registry.ReleaseAsset{Name: "plugin_v1.0.0_linux_amd64.tar.gz"},
			WasFallback:      false,
			RequestedVersion: "v1.0.0",
			FallbackReason:   "",
		}

		assert.False(t, info.WasFallback)
		assert.Equal(t, "v1.0.0", info.RequestedVersion)
		assert.Empty(t, info.FallbackReason)
		require.NotNil(t, info.Release)
		assert.Equal(t, "v1.0.0", info.Release.TagName)
	})

	t.Run("fallback scenario", func(t *testing.T) {
		info := registry.FallbackInfo{
			Release:          &registry.GitHubRelease{TagName: "v0.9.0"},
			Asset:            &registry.ReleaseAsset{Name: "plugin_v0.9.0_linux_amd64.tar.gz"},
			WasFallback:      true,
			RequestedVersion: "v1.0.0",
			FallbackReason:   "no compatible assets",
		}

		assert.True(t, info.WasFallback)
		assert.Equal(t, "v1.0.0", info.RequestedVersion)
		assert.Equal(t, "no compatible assets", info.FallbackReason)
		require.NotNil(t, info.Release)
		assert.Equal(t, "v0.9.0", info.Release.TagName)
	})

	t.Run("version mismatch when fallback", func(t *testing.T) {
		info := registry.FallbackInfo{
			Release:          &registry.GitHubRelease{TagName: "v0.9.0"},
			WasFallback:      true,
			RequestedVersion: "v1.0.0",
		}

		// Per data-model.md: If WasFallback=true, then Version != RequestedVersion
		if info.WasFallback {
			assert.NotEqual(t, info.RequestedVersion, info.Release.TagName)
		}
	})
}

// TestInstallOptionsExtended tests the extended InstallOptions struct.
func TestInstallOptionsExtended(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		opts := registry.InstallOptions{}

		assert.False(t, opts.Force)
		assert.False(t, opts.NoSave)
		assert.False(t, opts.FallbackToLatest)
		assert.False(t, opts.NoFallback)
		assert.Empty(t, opts.PluginDir)
	})

	t.Run("fallback to latest enabled", func(t *testing.T) {
		opts := registry.InstallOptions{
			FallbackToLatest: true,
		}

		assert.True(t, opts.FallbackToLatest)
		assert.False(t, opts.NoFallback)
	})

	t.Run("no fallback enabled", func(t *testing.T) {
		opts := registry.InstallOptions{
			NoFallback: true,
		}

		assert.False(t, opts.FallbackToLatest)
		assert.True(t, opts.NoFallback)
	})

	t.Run("mutual exclusivity validation", func(t *testing.T) {
		opts := registry.InstallOptions{
			FallbackToLatest: true,
			NoFallback:       true,
		}

		// Both flags set is invalid per spec - validation should be done by caller
		// This test documents the potential conflict state
		assert.True(t, opts.FallbackToLatest)
		assert.True(t, opts.NoFallback)
	})

	t.Run("combined with existing flags", func(t *testing.T) {
		opts := registry.InstallOptions{
			Force:            true,
			NoSave:           true,
			PluginDir:        "/custom/path",
			FallbackToLatest: true,
		}

		assert.True(t, opts.Force)
		assert.True(t, opts.NoSave)
		assert.Equal(t, "/custom/path", opts.PluginDir)
		assert.True(t, opts.FallbackToLatest)
	})
}

// TestInstallResultExtended tests the extended InstallResult struct.
func TestInstallResultExtended(t *testing.T) {
	t.Run("non-fallback result", func(t *testing.T) {
		result := registry.InstallResult{
			Name:             "test-plugin",
			Version:          "v1.0.0",
			Path:             "/home/user/.finfocus/plugins/test-plugin/v1.0.0",
			FromURL:          false,
			Repository:       "owner/repo",
			WasFallback:      false,
			RequestedVersion: "",
		}

		assert.Equal(t, "test-plugin", result.Name)
		assert.Equal(t, "v1.0.0", result.Version)
		assert.False(t, result.WasFallback)
		assert.Empty(t, result.RequestedVersion)
	})

	t.Run("fallback result", func(t *testing.T) {
		result := registry.InstallResult{
			Name:             "test-plugin",
			Version:          "v0.9.0",
			Path:             "/home/user/.finfocus/plugins/test-plugin/v0.9.0",
			FromURL:          false,
			Repository:       "owner/repo",
			WasFallback:      true,
			RequestedVersion: "v1.0.0",
		}

		assert.Equal(t, "v0.9.0", result.Version)
		assert.True(t, result.WasFallback)
		assert.Equal(t, "v1.0.0", result.RequestedVersion)
		// Per data-model.md: If WasFallback=true, then Version != RequestedVersion
		assert.NotEqual(t, result.Version, result.RequestedVersion)
	})

	t.Run("url-based install with fallback", func(t *testing.T) {
		result := registry.InstallResult{
			Name:             "custom-plugin",
			Version:          "v2.0.0",
			Path:             "/home/user/.finfocus/plugins/custom-plugin/v2.0.0",
			FromURL:          true,
			Repository:       "custom/repo",
			WasFallback:      true,
			RequestedVersion: "v2.1.0",
		}

		assert.True(t, result.FromURL)
		assert.True(t, result.WasFallback)
		assert.Equal(t, "v2.1.0", result.RequestedVersion)
	})

	t.Run("latest version request with fallback", func(t *testing.T) {
		result := registry.InstallResult{
			Name:             "test-plugin",
			Version:          "v0.9.0",
			Path:             "/home/user/.finfocus/plugins/test-plugin/v0.9.0",
			WasFallback:      true,
			RequestedVersion: "", // Empty when @latest was requested
		}

		assert.True(t, result.WasFallback)
		assert.Empty(t, result.RequestedVersion)
	})
}
