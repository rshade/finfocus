package registry

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestCustomRegistryCatalogCollisions(t *testing.T) {
	t.Parallel()
	available := []CustomPlugin{
		{Source: config.PluginRegistrySource{Name: "a"}, Entry: RegistryEntry{Name: "aws-public"}},
		{Source: config.PluginRegistrySource{Name: "a"}, Entry: RegistryEntry{Name: "shared"}},
		{Source: config.PluginRegistrySource{Name: "b"}, Entry: RegistryEntry{Name: "shared"}},
		{Source: config.PluginRegistrySource{Name: "b"}, Entry: RegistryEntry{Name: "unique"}},
	}
	plugins, err := filterCustomPlugins(available)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "unique", plugins[0].Entry.Name)
}

func TestCustomRegistryInstallRequiresConfirmationAndChecksum(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	const name = "provisioning"
	archive := createMockArchive(t, name)
	extension := ".tar.gz"
	if runtime.GOOS == "windows" {
		extension = ".zip"
	}
	asset := fmt.Sprintf("%s_v1.0.0_%s_%s%s", name, runtime.GOOS, runtime.GOARCH, extension)
	var includeChecksum atomic.Bool
	var checksums atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry.json":
			_, _ = fmt.Fprint(w, `{"schema_version":"1.0.0","plugins":{"provisioning":{
				"name":"provisioning","description":"Infrastructure plugin","repository":"company/provisioning",
				"author":"Company","supported_providers":["custom"],"min_spec_version":"0.1.0"}}}`)
		case "/repos/company/provisioning/releases/latest":
			origin := "https://" + r.Host
			assets := []ReleaseAsset{{Name: asset, BrowserDownloadURL: origin + "/bundle", Size: int64(len(archive))}}
			if includeChecksum.Load() {
				assets = append(assets, ReleaseAsset{Name: "checksums.txt", BrowserDownloadURL: origin + "/checksums"})
			}
			_ = json.NewEncoder(w).Encode(GitHubRelease{TagName: "v1.0.0", Assets: assets})
		case "/bundle":
			_, _ = w.Write(archive)
		case "/checksums":
			checksums.Add(1)
			_, _ = fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(archive), asset)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewCustomRegistryService(NewCustomIndexClient(server.Client()))
	source := config.PluginRegistrySource{Name: "workspace", URL: server.URL + "/registry.json"}
	require.NoError(t, service.Add(t.Context(), source))
	plugin, err := service.Resolve(t.Context(), source.Name, name)
	require.NoError(t, err)
	assert.Equal(t, config.PluginRegistryTrustCommunity, plugin.Entry.SecurityLevel)
	installer := NewInstallerWithClient(&GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}, t.TempDir())

	_, err = service.Install(t.Context(), installer, plugin, "", "", InstallOptions{}, nil)
	require.ErrorIs(t, err, ErrCustomRegistryConfirmation)
	_, err = service.Install(t.Context(), installer, plugin, "", source.Name,
		InstallOptions{SkipChecksum: true, AcceptMissingChecksum: true}, nil)
	require.ErrorIs(t, err, ErrChecksumUnavailable)

	includeChecksum.Store(true)
	result, err := service.Install(t.Context(), installer, plugin, "", "", InstallOptions{}, nil)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.Equal(t, int32(1), checksums.Load())
	record, err := config.GetInstalledPlugin(name)
	require.NoError(t, err)
	assert.Equal(t, source.Name, record.Registry)

	installer.recordInstalled = func(config.InstalledPlugin) error {
		return errors.New("disk full")
	}
	_, err = service.Install(t.Context(), installer, plugin, "", "", InstallOptions{Force: true}, nil)
	require.ErrorContains(t, err, "saving custom registry install")
	require.ErrorContains(t, err, "disk full")
	_, statErr := os.Stat(result.Path)
	require.NoError(t, statErr)

	require.NoError(t, os.RemoveAll(result.Path))
	_, err = service.Install(t.Context(), installer, plugin, "", "", InstallOptions{}, nil)
	require.ErrorContains(t, err, "saving custom registry install")
	_, statErr = os.Stat(result.Path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}
