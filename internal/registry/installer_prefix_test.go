package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// prefixedServer serves a monorepo whose release list interleaves CLI and plugin tags.
func prefixedServer(t *testing.T, pluginTags ...string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := func(tag string) GitHubRelease {
			canonical := CanonicalVersion(tag, "kubernetes-")
			name := testAssetName("finfocus-plugin-kubernetes", canonical)
			return GitHubRelease{TagName: tag, Assets: []ReleaseAsset{{
				Name:               name,
				BrowserDownloadURL: fmt.Sprintf("%s/download/%s", server.URL, name),
			}}}
		}
		switch {
		case r.URL.Path == "/repos/rshade/finfocus/releases":
			rels := []GitHubRelease{{TagName: "v0.3.9"}}
			for _, tag := range pluginTags {
				rels = append(rels, release(tag))
			}
			if !assert.NoError(t, json.NewEncoder(w).Encode(rels)) {
				return
			}
		case len(r.URL.Path) > len("/repos/rshade/finfocus/releases/tags/") &&
			r.URL.Path[:len("/repos/rshade/finfocus/releases/tags/")] == "/repos/rshade/finfocus/releases/tags/":
			tag := filepath.Base(r.URL.Path)
			for _, pt := range pluginTags {
				if pt == tag {
					assert.NoError(t, json.NewEncoder(w).Encode(release(tag)))
					return
				}
			}
			http.NotFound(w, r)
		case filepath.Dir(r.URL.Path) == "/download":
			_, _ = w.Write(createMockArchive(t, "finfocus-plugin-kubernetes"))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return server
}

func newPrefixedInstaller(t *testing.T, server *httptest.Server) (*Installer, string) {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("FINFOCUS_HOME", filepath.Join(tmpHome, ".finfocus"))
	config.ResetGlobalConfigForTest()
	config.InitGlobalConfig()
	pluginDir := filepath.Join(tmpHome, ".finfocus", "plugins")
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	return NewInstallerWithClient(client, pluginDir), pluginDir
}

//nolint:gochecknoglobals // Shared read-only test fixture reused across this file's test cases.
var kubernetesHints = &AssetNamingHints{
	AssetPrefix: "finfocus-plugin-kubernetes",
	TagPrefix:   "kubernetes-",
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via newPrefixedInstaller)
func TestInstallRelease_PrefixedTagUsesCanonicalDir(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	inst, pluginDir := newPrefixedInstaller(t, server)

	rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", "", kubernetesHints)
	require.NoError(t, err)
	res, err := inst.installRelease(context.Background(), "kubernetes", rel, "rshade/finfocus",
		InstallOptions{}, nil, kubernetesHints)
	require.NoError(t, err)

	assert.Equal(t, "v0.1.0", res.Version)
	_, statErr := os.Stat(filepath.Join(pluginDir, "kubernetes", "v0.1.0"))
	require.NoError(t, statErr)

	plugins, warnings, err := (&Registry{root: pluginDir}).ListLatestPlugins()
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, plugins, 1, "semver dir must be discoverable")
	assert.Equal(t, "v0.1.0", plugins[0].Version)
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via newPrefixedInstaller)
func TestFetchRelease_BareVersionResolvesPrefixedTag(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0", "kubernetes-v0.2.0")
	defer server.Close()
	inst, _ := newPrefixedInstaller(t, server)

	for _, v := range []string{"0.1.0", "v0.1.0", "kubernetes-v0.1.0"} {
		rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", v, kubernetesHints)
		require.NoError(t, err, v)
		assert.Equal(t, "kubernetes-v0.1.0", rel.TagName, v)
	}
}

func TestFindReleaseWithFallbackInfo_PrefixFiltersCoreReleases(t *testing.T) {
	t.Parallel()

	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	c := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}

	info, err := c.FindReleaseWithFallbackInfo(context.Background(), "rshade", "finfocus",
		"v9.9.9", "kubernetes", kubernetesHints)
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.0", info.Release.TagName)
	assert.True(t, info.WasFallback)
}

// TestCompareVersions_PrefixedCanonicalVersions verifies that the canonical
// version derived from a prefixed monorepo tag compares equal to the installed
// version string, so an unchanged release would not trigger a reinstall.
// It exercises fetchRelease/installRelease directly; Installer.Update itself is
// covered end-to-end by TestUpdate_PrefixedPluginLifecycle.
//
//nolint:paralleltest // t.Setenv changes the process-wide environment (via newPrefixedInstaller)
func TestCompareVersions_PrefixedCanonicalVersions(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	inst, _ := newPrefixedInstaller(t, server)

	rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", "", kubernetesHints)
	require.NoError(t, err)
	_, err = inst.installRelease(context.Background(), "kubernetes", rel, "rshade/finfocus",
		InstallOptions{}, nil, kubernetesHints)
	require.NoError(t, err)

	newVersion := CanonicalVersion(rel.TagName, kubernetesHints.TagPrefix)
	cmp, err := CompareVersions(newVersion, "v0.1.0")
	require.NoError(t, err, "canonical versions must be comparable")
	assert.Equal(t, 0, cmp, "same version must not trigger reinstall")
}

// TestUpdate_PrefixedPluginLifecycle exercises Installer.Update end-to-end for
// the kubernetes monorepo plugin: install at v0.1.0 through the embedded
// registry entry, update when a newer kubernetes-vX.Y.Z tag exists, and no-op
// once current.
//
//nolint:paralleltest // t.Setenv changes the process-wide environment (via newPrefixedInstaller)
func TestUpdate_PrefixedPluginLifecycle(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0", "kubernetes-v0.2.0")
	defer server.Close()
	inst, pluginDir := newPrefixedInstaller(t, server)
	ctx := context.Background()

	res, err := inst.Install(ctx, "kubernetes@v0.1.0", InstallOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", res.Version)

	upd, err := inst.Update(ctx, "kubernetes", UpdateOptions{}, nil)
	require.NoError(t, err)
	assert.False(t, upd.WasUpToDate)
	assert.Equal(t, "v0.1.0", upd.OldVersion)
	assert.Equal(t, "v0.2.0", upd.NewVersion)

	_, statErr := os.Stat(filepath.Join(pluginDir, "kubernetes", "v0.2.0"))
	require.NoError(t, statErr, "new version dir must exist")
	_, statErr = os.Stat(filepath.Join(pluginDir, "kubernetes", "v0.1.0"))
	assert.True(t, os.IsNotExist(statErr), "old version dir must be removed")

	installed, err := config.GetInstalledPlugin("kubernetes")
	require.NoError(t, err)
	assert.Equal(t, "v0.2.0", installed.Version)

	again, err := inst.Update(ctx, "kubernetes", UpdateOptions{}, nil)
	require.NoError(t, err)
	assert.True(t, again.WasUpToDate, "already-current plugin must be a no-op")
	assert.Equal(t, "v0.2.0", again.OldVersion)
	assert.Equal(t, "v0.2.0", again.NewVersion)
}
