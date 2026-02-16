package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestInstall_FromRegistry(t *testing.T) {
	// Setup config for test
	config.ResetGlobalConfigForTest()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	configDir := filepath.Join(tmpHome, ".finfocus")
	_ = os.MkdirAll(configDir, 0755)

	// Initialize global config (needed for AddInstalledPlugin).
	config.InitGlobalConfig()

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/rshade/finfocus-plugin-aws-public/releases/latest" {
			// Return release info
			ext := ".tar.gz"
			if runtime.GOOS == "windows" {
				ext = ".zip"
			}
			assetName := fmt.Sprintf("aws-public_v1.0.0_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
			downloadURL := fmt.Sprintf("%s/download/%s", "http://"+r.Host, assetName)

			release := GitHubRelease{
				TagName: "v1.0.0",
				Name:    "v1.0.0",
				Assets: []ReleaseAsset{
					{
						Name:               assetName,
						Size:               1024,
						BrowserDownloadURL: downloadURL,
						ContentType:        "application/octet-stream",
					},
				},
			}
			json.NewEncoder(w).Encode(release)
			return
		}

		// Match download URL
		if r.URL.Path == fmt.Sprintf(
			"/download/aws-public_v1.0.0_%s_%s.tar.gz",
			runtime.GOOS,
			runtime.GOARCH,
		) ||
			r.URL.Path == fmt.Sprintf("/download/aws-public_v1.0.0_%s_%s.zip", runtime.GOOS, runtime.GOARCH) {
			// Return asset content
			w.Write(createMockArchive(t, "aws-public"))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	// Setup installer
	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL

	pluginDir := filepath.Join(tmpHome, "plugins")
	installer := NewInstallerWithClient(client, pluginDir)

	// Install
	result, err := installer.Install(context.Background(), "aws-public", InstallOptions{}, nil)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if result.Name != "aws-public" {
		t.Errorf("Expected name aws-public, got %s", result.Name)
	}
	if result.Version != "v1.0.0" {
		t.Errorf("Expected version v1.0.0, got %s", result.Version)
	}

	// Verify file exists
	binaryPath := filepath.Join(pluginDir, "aws-public", "v1.0.0", "aws-public")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Errorf("Binary not found at %s", binaryPath)
	}

	// Verify config updated
	plugin, err := config.GetInstalledPlugin("aws-public")
	if err != nil {
		t.Errorf("Plugin not found in config: %v", err)
	}
	if plugin.Version != "v1.0.0" {
		t.Errorf("Expected config version v1.0.0, got %s", plugin.Version)
	}
}

func TestRemove(t *testing.T) {
	// Setup
	config.ResetGlobalConfigForTest()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	config.InitGlobalConfig()

	pluginDir := filepath.Join(tmpHome, "plugins")
	installer := NewInstaller(pluginDir)

	// Manually "install" a plugin.
	name := "test-plugin"
	version := "v1.0.0"
	installPath := filepath.Join(pluginDir, name, version)
	if err := os.MkdirAll(installPath, 0755); err != nil {
		t.Fatalf("Failed to create install path: %v", err)
	}

	// Add to config.
	if err := config.AddInstalledPlugin(config.InstalledPlugin{
		Name:    name,
		Version: version,
		URL:     "github.com/owner/repo",
	}); err != nil {
		t.Fatalf("Failed to add installed plugin: %v", err)
	}

	// Remove
	err := installer.Remove(name, RemoveOptions{}, nil)
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify directory gone
	if _, err := os.Stat(installPath); !os.IsNotExist(err) {
		t.Error("Plugin directory still exists")
	}

	// Verify config updated
	_, err = config.GetInstalledPlugin(name)
	if err == nil {
		t.Error("Plugin still exists in config")
	}
}

// createMockArchive creates a mock archive with a binary inside.
func createMockArchive(t *testing.T, binaryName string) []byte {
	tmpDir := t.TempDir()

	// Create binary content
	binName := binaryName
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	content := []byte("mock binary content")

	archivePath := filepath.Join(tmpDir, "archive")
	if runtime.GOOS == "windows" {
		archivePath += ".zip"
		err := createZip(archivePath, binName, content)
		if err != nil {
			t.Fatalf("Failed to create zip: %v", err)
		}
	} else {
		archivePath += ".tar.gz"
		err := createTarGz(archivePath, binName, content)
		if err != nil {
			t.Fatalf("Failed to create tar.gz: %v", err)
		}
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Failed to read archive: %v", err)
	}
	return data
}

func createTarGz(path, filename string, content []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := tw.Write(content); err != nil {
		return err
	}
	return nil
}

func createZip(path, filename string, content []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	w, err := zw.Create(filename)
	if err != nil {
		return err
	}
	if _, err := w.Write(content); err != nil {
		return err
	}
	return nil
}

// checksumTestEnv holds common test infrastructure for checksum verification tests.
type checksumTestEnv struct {
	tmpHome   string
	pluginDir string
	installer *Installer
	client    *GitHubClient
}

// setupChecksumTest creates the common test infrastructure for all checksum
// verification tests: resets config, creates temp HOME with .finfocus dir,
// initializes global config, and returns an environment with a GitHubClient
// and Installer ready for use. The caller must set client.HTTPClient and
// client.BaseURL to point at their httptest.Server.
func setupChecksumTest(t *testing.T) *checksumTestEnv {
	t.Helper()
	config.ResetGlobalConfigForTest()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	_ = os.MkdirAll(filepath.Join(tmpHome, ".finfocus"), 0755)
	config.InitGlobalConfig()

	client := NewGitHubClient()
	pluginDir := filepath.Join(tmpHome, "plugins")
	installer := NewInstallerWithClient(client, pluginDir)

	return &checksumTestEnv{
		tmpHome:   tmpHome,
		pluginDir: pluginDir,
		installer: installer,
		client:    client,
	}
}

// checksumTestAssetName returns the platform-specific asset name for checksum tests.
func checksumTestAssetName(pluginName, version string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s%s", pluginName, version, runtime.GOOS, runtime.GOARCH, ext)
}

// checksumForBytes computes the SHA256 hex string for raw bytes.
func checksumForBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestInstall_ChecksumVerified(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "checksum-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)
	correctHash := checksumForBytes(archiveData)
	checksumsContent := fmt.Sprintf("%s  %s\n", correctHash, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/checksum-plugin/releases/tags/" + version:
			release := GitHubRelease{
				TagName: version,
				Name:    version,
				Assets: []ReleaseAsset{
					{
						Name:               assetName,
						Size:               int64(len(archiveData)),
						BrowserDownloadURL: "http://" + r.Host + "/download/" + assetName,
					},
					{
						Name:               "checksums.txt",
						Size:               int64(len(checksumsContent)),
						BrowserDownloadURL: "http://" + r.Host + "/download/checksums.txt",
					},
				},
			}
			_ = json.NewEncoder(w).Encode(release)
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(checksumsContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               int64(len(checksumsContent)),
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/checksum-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		progress,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, pluginName, result.Name)
	assert.Equal(t, version, result.Version)

	// Verify "Checksum verified" appeared in progress messages
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "Checksum verified") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'Checksum verified' in progress messages, got: %v", messages)
}

func TestInstall_ChecksumMismatchBlocksInstallation(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "checksum-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)
	wrongHash := strings.Repeat("ab", 32)
	checksumsContent := fmt.Sprintf("%s  %s\n", wrongHash, assetName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(checksumsContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	_, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               int64(len(checksumsContent)),
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/checksum-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		nil,
		nil,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")

	// Verify install directory was NOT created
	installDir := filepath.Join(env.pluginDir, pluginName, version)
	_, statErr := os.Stat(installDir)
	assert.True(t, os.IsNotExist(statErr), "install directory should not exist after checksum mismatch")
}

func TestInstall_NoChecksumsAssetWarns(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "no-checksum-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download/"+assetName {
			_, _ = w.Write(archiveData)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				// No checksums.txt asset
			},
		},
		"owner/no-checksum-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		progress,
		nil,
	)

	require.NoError(t, err, "installation should succeed without checksums.txt")
	assert.Equal(t, pluginName, result.Name)

	// Verify warning was emitted
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "checksums.txt not found") || strings.Contains(msg, "skipping verification") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about missing checksums.txt, got: %v", messages)
}

func TestInstall_ChecksumsAssetNotListed(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "unlisted-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)
	// checksums.txt lists a different asset, not the one we're downloading
	checksumsContent := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  other-file.tar.gz\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(checksumsContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               int64(len(checksumsContent)),
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/unlisted-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		progress,
		nil,
	)

	require.NoError(t, err, "installation should succeed when asset not listed in checksums")
	assert.Equal(t, pluginName, result.Name)

	// Verify warning was emitted
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "not listed") || strings.Contains(msg, "not found in checksums") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about asset not listed in checksums, got: %v", messages)
}

func TestInstall_ChecksumsDownloadFails(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "download-fail-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			// Simulate server error
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               100,
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/download-fail-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		progress,
		nil,
	)

	require.NoError(t, err, "installation should succeed when checksums download fails")
	assert.Equal(t, pluginName, result.Name)

	// Verify warning was emitted about download failure
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "Warning") && strings.Contains(msg, "checksum") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about checksums download failure, got: %v", messages)
}

func TestInstall_MalformedChecksumsWarns(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "malformed-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)
	malformedContent := "this is not a valid checksums file\nno hashes here\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			_, _ = w.Write([]byte(malformedContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               int64(len(malformedContent)),
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/malformed-plugin",
		InstallOptions{PluginDir: env.pluginDir},
		progress,
		nil,
	)

	require.NoError(t, err, "installation should succeed with malformed checksums")
	assert.Equal(t, pluginName, result.Name)

	// Verify warning was emitted about malformed checksums
	found := false
	for _, msg := range messages {
		if strings.Contains(msg, "malformed") || strings.Contains(msg, "checksum") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected warning about malformed checksums, got: %v", messages)
}

func TestInstall_SkipChecksumBypassesVerification(t *testing.T) {
	env := setupChecksumTest(t)

	pluginName := "skip-plugin"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archiveData := createMockArchive(t, pluginName)
	// Use a wrong hash - but SkipChecksum should bypass verification
	wrongHash := strings.Repeat("ab", 32)
	checksumsContent := fmt.Sprintf("%s  %s\n", wrongHash, assetName)

	var checksumDownloaded atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/" + assetName:
			_, _ = w.Write(archiveData)
		case "/download/checksums.txt":
			checksumDownloaded.Store(true)
			_, _ = w.Write([]byte(checksumsContent))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	env.client.HTTPClient = server.Client()
	env.client.BaseURL = server.URL

	var messages []string
	progress := func(msg string) {
		messages = append(messages, msg)
	}

	result, err := env.installer.installRelease(
		context.Background(),
		pluginName,
		&GitHubRelease{
			TagName: version,
			Assets: []ReleaseAsset{
				{
					Name:               assetName,
					Size:               int64(len(archiveData)),
					BrowserDownloadURL: server.URL + "/download/" + assetName,
				},
				{
					Name:               "checksums.txt",
					Size:               int64(len(checksumsContent)),
					BrowserDownloadURL: server.URL + "/download/checksums.txt",
				},
			},
		},
		"owner/skip-plugin",
		InstallOptions{PluginDir: env.pluginDir, SkipChecksum: true},
		progress,
		nil,
	)

	require.NoError(t, err, "installation should succeed with SkipChecksum=true even with wrong hash")
	assert.Equal(t, pluginName, result.Name)
	assert.False(t, checksumDownloaded.Load(), "checksums.txt should not be downloaded when SkipChecksum is true")

	// Should NOT contain checksum-related messages
	for _, msg := range messages {
		assert.NotContains(t, msg, "Checksum verified")
		assert.NotContains(t, msg, "checksum mismatch")
	}
}
