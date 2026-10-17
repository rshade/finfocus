package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall_CatalogOnlyRejectsNonNames(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "catalog policy reached the network", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	installer := NewInstallerWithClient(client, t.TempDir())

	for _, specifier := range []string{"github.com/owner/repo", "owner/repo", "github.com/owner/repo@v1.2.3"} {
		t.Run(specifier, func(t *testing.T) {
			t.Parallel()
			_, err := installer.Install(context.Background(), specifier, InstallOptions{CatalogOnly: true}, nil)
			require.ErrorIs(t, err, ErrCatalogSpecifier)
		})
	}
	assert.Equal(t, int32(0), hits.Load())
}

func TestInstall_CatalogOnlyIgnoresSkipChecksum(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	installer := policyInstaller(t, pluginDir, policyRelease{
		plugin:  "aws-public",
		version: "v1.0.0",
		hash:    "0000000000000000000000000000000000000000000000000000000000000000",
	})

	_, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{
		CatalogOnly:  true,
		SkipChecksum: true,
		NoSave:       true,
	}, nil)
	require.ErrorIs(t, err, ErrChecksumMismatch)
	_, statErr := os.Stat(filepath.Join(pluginDir, "aws-public", "v1.0.0"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInstall_RequireChecksumFile(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	installer := policyInstaller(t, pluginDir, policyRelease{
		plugin:       "aws-public",
		version:      "v1.0.0",
		omitChecksum: true,
	})

	_, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{
		RequireChecksumFile: true,
		NoSave:              true,
	}, nil)
	require.ErrorIs(t, err, ErrChecksumUnavailable)
	_, statErr := os.Stat(filepath.Join(pluginDir, "aws-public", "v1.0.0"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInstall_AcceptMissingChecksumContinues(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	installer := policyInstaller(t, pluginDir, policyRelease{
		plugin:       "aws-public",
		version:      "v1.0.0",
		omitChecksum: true,
	})

	var messages []string
	result, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{
		RequireChecksumFile:   true,
		AcceptMissingChecksum: true,
		NoSave:                true,
	}, func(msg string) { messages = append(messages, msg) })
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.Contains(t, messages, "Warning: checksums.txt not found in release, skipping verification")
}

func TestInstall_CatalogOnlyRefusesSilentFallback(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	var downloads atomic.Int32
	installer := fallbackInstaller(t, pluginDir, &downloads)

	_, err := installer.Install(context.Background(), "aws-public", InstallOptions{
		CatalogOnly: true,
		NoSave:      true,
	}, nil)
	require.Error(t, err)
	var noAssets *NoAssetsError
	require.ErrorAs(t, err, &noAssets)
	assert.Equal(t, "v2.0.0", noAssets.Version)
	assert.Equal(t, int32(0), downloads.Load())
	_, statErr := os.Stat(filepath.Join(pluginDir, "aws-public", "v1.0.0"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInstall_CatalogOnlyAcceptsNamedFallback(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	var downloads atomic.Int32
	installer := fallbackInstaller(t, pluginDir, &downloads)

	result, err := installer.Install(context.Background(), "aws-public", InstallOptions{
		CatalogOnly:           true,
		AcceptFallbackVersion: "v1.0.0",
		NoSave:                true,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.Positive(t, downloads.Load())
}

func TestInstall_DefaultStillSwapsAssetlessLatest(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	var downloads atomic.Int32
	installer := fallbackInstaller(t, pluginDir, &downloads)

	var messages []string
	result, err := installer.Install(
		context.Background(),
		"aws-public",
		InstallOptions{NoSave: true},
		func(msg string) {
			messages = append(messages, msg)
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Version)
	assert.Contains(
		t,
		messages,
		"Warning: latest release v2.0.0 has no assets yet (the upload may still be in progress); installing v1.0.0 instead",
	)
}

func TestAcquireLock_BusyMatchesSentinel(t *testing.T) {
	t.Parallel()

	installer := NewInstaller(t.TempDir())
	unlock, err := installer.acquireLock("demo")
	require.NoError(t, err)
	t.Cleanup(unlock)

	_, err = installer.acquireLock("demo")
	require.ErrorIs(t, err, ErrPluginBusy)
	assert.Equal(t,
		fmt.Sprintf("plugin %q is currently being modified by another process (lock file %s exists)",
			"demo", filepath.Join(installer.pluginDir, "demo.lock")),
		err.Error())
}

type policyRelease struct {
	plugin       string
	version      string
	hash         string
	omitChecksum bool
}

func policyInstaller(t *testing.T, pluginDir string, rel policyRelease) *Installer {
	t.Helper()
	archive := createMockArchive(t, rel.plugin)
	assetName := checksumTestAssetName(rel.plugin, rel.version)
	checksums := rel.hash + "  " + assetName + "\n"
	if rel.hash == "" && !rel.omitChecksum {
		checksums = checksumForBytes(archive) + "  " + assetName + "\n"
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/rshade/finfocus-plugin-aws-public/releases/tags/" + rel.version:
			assets := []ReleaseAsset{{
				Name:               assetName,
				Size:               int64(len(archive)),
				BrowserDownloadURL: "http://" + r.Host + "/download/" + assetName,
			}}
			if !rel.omitChecksum {
				assets = append(assets, ReleaseAsset{
					Name:               "checksums.txt",
					Size:               int64(len(checksums)),
					BrowserDownloadURL: "http://" + r.Host + "/download/checksums.txt",
				})
			}
			assert.NoError(t, json.NewEncoder(w).Encode(GitHubRelease{
				TagName: rel.version,
				Name:    rel.version,
				Assets:  assets,
			}))
		case "/download/" + assetName:
			_, err := w.Write(archive)
			assert.NoError(t, err)
		case "/download/checksums.txt":
			_, err := w.Write([]byte(checksums))
			assert.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	return NewInstallerWithClient(client, pluginDir)
}

func fallbackInstaller(t *testing.T, pluginDir string, downloads *atomic.Int32) *Installer {
	t.Helper()
	archive := createMockArchive(t, "aws-public")
	assetName := checksumTestAssetName("aws-public", "v1.0.0")
	checksums := checksumForBytes(archive) + "  " + assetName + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/rshade/finfocus-plugin-aws-public/releases/latest":
			assert.NoError(t, json.NewEncoder(w).Encode(GitHubRelease{TagName: "v2.0.0", Name: "v2.0.0"}))
		case "/repos/rshade/finfocus-plugin-aws-public/releases":
			assert.NoError(t, json.NewEncoder(w).Encode([]GitHubRelease{
				{TagName: "v2.0.0", Name: "v2.0.0"},
				{
					TagName: "v1.0.0",
					Name:    "v1.0.0",
					Assets: []ReleaseAsset{
						{
							Name:               assetName,
							Size:               int64(len(archive)),
							BrowserDownloadURL: "http://" + r.Host + "/download/" + assetName,
						},
						{
							Name:               "checksums.txt",
							Size:               int64(len(checksums)),
							BrowserDownloadURL: "http://" + r.Host + "/download/checksums.txt",
						},
					},
				},
			}))
		case "/download/" + assetName:
			downloads.Add(1)
			_, err := w.Write(archive)
			assert.NoError(t, err)
		case "/download/checksums.txt":
			_, err := w.Write([]byte(checksums))
			assert.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	return NewInstallerWithClient(client, pluginDir)
}

func TestInstall_WrongAcceptFallbackStaysNoAssets(t *testing.T) {
	t.Parallel()

	pluginDir := t.TempDir()
	var downloads atomic.Int32
	installer := fallbackInstaller(t, pluginDir, &downloads)
	_, err := installer.Install(context.Background(), "aws-public", InstallOptions{
		CatalogOnly:           true,
		AcceptFallbackVersion: "v0.9.0",
		NoSave:                true,
	}, nil)
	require.Error(t, err)
	var noAssets *NoAssetsError
	require.ErrorAs(t, err, &noAssets)
	assert.Equal(t, int32(0), downloads.Load())
	assert.NotErrorIs(t, err, ErrCatalogSpecifier)
}

func TestVerifyChecksum_WebPolicyRejectsLookupFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/checksums.txt":
			http.Error(w, "unavailable", http.StatusInternalServerError)
		case "/checksums-missing-entry.txt":
			_, _ = io.WriteString(w, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  other.tar.gz\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	installer := NewInstallerWithClient(client, t.TempDir())
	archive := filepath.Join(t.TempDir(), "plugin.tar.gz")
	require.NoError(t, os.WriteFile(archive, []byte("plugin"), 0o600))
	asset := &ReleaseAsset{Name: "plugin.tar.gz"}

	downloadFailed := &GitHubRelease{Assets: []ReleaseAsset{{
		Name:               "checksums.txt",
		BrowserDownloadURL: server.URL + "/checksums.txt",
	}}}
	err := installer.verifyChecksumForRelease(
		context.Background(), downloadFailed, asset, archive,
		InstallOptions{RequireChecksumFile: true}, newProgressReport(nil, nil),
	)
	require.Error(t, err)

	err = installer.verifyChecksumForRelease(
		context.Background(), downloadFailed, asset, archive,
		InstallOptions{}, newProgressReport(nil, nil),
	)
	require.NoError(t, err)

	missingEntry := &GitHubRelease{Assets: []ReleaseAsset{{
		Name:               "checksums.txt",
		BrowserDownloadURL: server.URL + "/checksums-missing-entry.txt",
	}}}
	err = installer.verifyChecksumForRelease(
		context.Background(), missingEntry, asset, archive,
		InstallOptions{RequireChecksumFile: true, AcceptMissingChecksum: true}, newProgressReport(nil, nil),
	)
	require.Error(t, err)
}
