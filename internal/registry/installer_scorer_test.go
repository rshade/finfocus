package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// The HTTP fixture stands in for future published releases; these tests do not
// claim scorer assets are already available on the public repository.
func newScorerInstallFixture(t *testing.T, publishScorer bool) (*Installer, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FINFOCUS_HOME", filepath.Join(home, "finfocus"))
	t.Setenv("FINFOCUS_PLUGIN_DIR", "")
	t.Setenv("FINFOCUS_CATALOG_FILE", "")

	scorerAsset := testAssetName("finfocus-plugin-scorer", "v0.1.0")
	jevAsset := testAssetName("finfocus-plugin-jev", "v0.1.4")
	archives := map[string][]byte{
		"/download/" + scorerAsset: createMockArchive(t, "finfocus-plugin-scorer"),
		"/download/" + jevAsset:    createMockArchive(t, "finfocus-plugin-jev"),
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := archives[r.URL.Path]; ok {
			_, err := w.Write(data)
			assert.NoError(t, err)
			return
		}
		if r.URL.Path != "/repos/rshade/finfocus/releases" {
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		releases := []GitHubRelease{
			{TagName: "v99.0.0", Assets: []ReleaseAsset{{Name: "finfocus-core.tar.gz"}}},
			{TagName: "jev-v0.1.4", Assets: []ReleaseAsset{{
				Name: jevAsset, BrowserDownloadURL: server.URL + "/download/" + jevAsset,
			}}},
			{TagName: "kubernetes-v99.0.0"},
		}
		if publishScorer {
			releases = append(releases, GitHubRelease{TagName: "scorer-v0.1.0", Assets: []ReleaseAsset{{
				Name: scorerAsset, BrowserDownloadURL: server.URL + "/download/" + scorerAsset,
			}}})
		}
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(releases))
	}))
	t.Cleanup(server.Close)
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	return NewInstallerWithClient(client, ""), filepath.Join(home, "finfocus", "plugins")
}

//nolint:paralleltest // Fixture isolates process-wide environment with t.Setenv.
func TestInstallScorer_CleanHomeDiscoversScorerRelease(t *testing.T) {
	installer, pluginDir := newScorerInstallFixture(t, true)
	before, err := NewDefault().ListPlugins()
	require.NoError(t, err)
	require.Empty(t, before)

	result, err := installer.Install(context.Background(), "scorer", InstallOptions{}, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "scorer", result.Name)
	assert.Equal(t, "v0.1.0", result.Version)
	assert.Equal(t, "rshade/finfocus", result.Repository)
	assert.Equal(t, filepath.Join(pluginDir, "scorer", "v0.1.0"), result.Path)
	assert.False(t, result.WasFallback)

	binaryName := "finfocus-plugin-scorer"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	found, err := NewDefault().ListPlugins()
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "scorer", found[0].Name)
	assert.Equal(t, "v0.1.0", found[0].Version)
	assert.Equal(t, filepath.Join(result.Path, binaryName), found[0].Path)
	assert.FileExists(t, found[0].Path)
	assert.NoDirExists(t, filepath.Join(pluginDir, "jev"))

	saved, err := config.LoadInstalledPlugins()
	require.NoError(t, err)
	assert.Equal(t, []config.InstalledPlugin{{
		Name: "scorer", URL: "github.com/rshade/finfocus", Version: "v0.1.0",
	}}, saved)

	_, err = installer.Install(context.Background(), "scorer", InstallOptions{}, nil)
	require.ErrorContains(t, err, "already installed")
	_, err = installer.Install(context.Background(), "scorer", InstallOptions{Force: true}, nil)
	require.NoError(t, err)
	saved, err = config.LoadInstalledPlugins()
	require.NoError(t, err)
	assert.Len(t, saved, 1, "forced reinstall must not duplicate the saved plugin")
}

//nolint:paralleltest // Fixture isolates process-wide environment with t.Setenv.
func TestInstallScorer_UnpublishedReleaseDoesNotInstallJevOrCore(t *testing.T) {
	installer, pluginDir := newScorerInstallFixture(t, false)
	result, err := installer.Install(context.Background(), "scorer", InstallOptions{}, nil)
	require.ErrorContains(t, err, `no stable release with tag prefix "scorer-" found`)
	assert.Nil(t, result)
	assert.NoDirExists(t, filepath.Join(pluginDir, "scorer"))
	found, err := NewDefault().ListPlugins()
	require.NoError(t, err)
	assert.Empty(t, found)
	saved, err := config.LoadInstalledPlugins()
	require.NoError(t, err)
	assert.Empty(t, saved)
}

//nolint:paralleltest // Fixture isolates process-wide environment with t.Setenv.
func TestInstallScorer_LegacyJevRemainsSeparate(t *testing.T) {
	installer, pluginDir := newScorerInstallFixture(t, true)
	legacy, err := installer.Install(context.Background(), "jev", InstallOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "jev", legacy.Name)
	assert.Equal(t, "v0.1.4", legacy.Version)
	assert.Equal(t, filepath.Join(pluginDir, "jev", "v0.1.4"), legacy.Path)

	_, err = installer.Install(context.Background(), "scorer", InstallOptions{}, nil)
	require.NoError(t, err)
	found, err := NewDefault().ListPlugins()
	require.NoError(t, err)
	require.Len(t, found, 2)
	assert.ElementsMatch(t, []string{"jev", "scorer"}, []string{found[0].Name, found[1].Name})
	assert.DirExists(t, legacy.Path)
	_, err = installer.Install(context.Background(), "jev", InstallOptions{}, nil)
	require.ErrorContains(t, err, "already installed")

	saved, err := config.LoadInstalledPlugins()
	require.NoError(t, err)
	assert.ElementsMatch(t, []config.InstalledPlugin{
		{Name: "jev", URL: "github.com/rshade/finfocus", Version: "v0.1.4"},
		{Name: "scorer", URL: "github.com/rshade/finfocus", Version: "v0.1.0"},
	}, saved)
}
