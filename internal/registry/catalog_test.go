package registry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeCatalog_IncludesOutsideInstallWithoutNetwork(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writePluginBinary(t, root, "aws-public", "v1.0.0")
	writePluginBinary(t, root, "local-tool", "v0.3.0")
	installed, err := New(root).ListPlugins()
	require.NoError(t, err)

	var lookups []string
	rows := MergeCatalog([]RegistryEntry{{
		Name:               "aws-public",
		Description:        "AWS",
		Repository:         "rshade/finfocus-plugin-aws-public",
		SecurityLevel:      "community",
		SupportedProviders: []string{"aws"},
		Capabilities:       []string{"projected_cost"},
	}}, installed, func(entry RegistryEntry) (string, error) {
		lookups = append(lookups, entry.Name)
		if entry.Name == "aws-public" {
			return "v2.0.0", nil
		}
		return "", errors.New("network disabled")
	})

	require.Len(t, rows, 2)
	assert.Equal(t, []string{"aws-public"}, lookups)
	assert.Equal(t, "aws-public", rows[0].Name)
	assert.True(t, rows[0].InCatalog)
	assert.Equal(t, []string{"v1.0.0"}, rows[0].InstalledVersions)
	assert.Equal(t, "v2.0.0", rows[0].LatestVersion)
	assert.True(t, rows[0].UpdateAvailable)
	assert.Equal(t, "community", rows[0].SecurityLevel)

	assert.Equal(t, "local-tool", rows[1].Name)
	assert.False(t, rows[1].InCatalog)
	assert.Equal(t, []string{"v0.3.0"}, rows[1].InstalledVersions)
	assert.Empty(t, rows[1].Repository)
	assert.Empty(t, rows[1].LatestVersion)
	assert.False(t, rows[1].UpdateAvailable)
}

func TestMergeCatalog_LatestFailureIsARowWarning(t *testing.T) {
	t.Parallel()
	rows := MergeCatalog([]RegistryEntry{{Name: "aws-public"}}, nil, func(RegistryEntry) (string, error) {
		return "", errors.New("rate limit")
	})
	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].LatestVersion)
	assert.False(t, rows[0].UpdateAvailable)
	assert.NotEmpty(t, rows[0].Warning)
	assert.NotNil(t, rows[0].InstalledVersions)
}

func TestMergeCatalog_CopiesDeprecation(t *testing.T) {
	t.Parallel()
	rows := MergeCatalog([]RegistryEntry{{
		Name:        "jev",
		Deprecated:  true,
		Replacement: "scorer",
	}}, nil, nil)
	require.Len(t, rows, 1)
	assert.True(t, rows[0].Deprecated)
	assert.Equal(t, "scorer", rows[0].Replacement)
}

func writePluginBinary(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, name, version)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755))
}

func TestVersionIsNewerComparesTheNewestInstall(t *testing.T) {
	t.Parallel()
	assert.False(t, versionIsNewer("2.0.0", []string{"1.0.0", "2.0.0"}))
	assert.False(t, versionIsNewer("1.5.0", []string{"2.0.0", "1.0.0"}))
	assert.True(t, versionIsNewer("2.1.0", []string{"1.0.0", "2.0.0"}))
}

func TestCarbonCatalogMetadata(t *testing.T) {
	t.Parallel()
	entry, err := GetPlugin("carbon")
	require.NoError(t, err)
	assert.Equal(t, "rshade/finfocus", entry.Repository)
	assert.Equal(t, "carbon-", entry.TagPrefix)
	assert.Equal(t, []string{"carbon"}, entry.Capabilities)
	require.NotNil(t, entry.AssetHints)
	assert.Equal(t, "finfocus-plugin-carbon", entry.AssetHints.AssetPrefix)
	require.NoError(t, ValidateRegistryEntry(*entry))
}
