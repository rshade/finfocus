package plugin_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockRelease matches GitHub API release structure.
type MockRelease struct {
	TagName string      `json:"tag_name"`
	Assets  []MockAsset `json:"assets"`
}

// MockAsset matches GitHub API asset structure.
type MockAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// StartMockRegistry creates a test server that mocks GitHub Release API and file downloads.
// It returns the server and a cleanup function. Routes handled include:
// GET /repos/{owner}/{repo}/releases/latest, GET /repos/{owner}/{repo}/releases/tags/{tag},
// and GET /download/{filename}.
func StartMockRegistry(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	// Track created artifacts to serve them later
	var mu sync.Mutex
	artifacts := make(map[string][]byte)

	// Helper to create a release response
	createRelease := func(tagName string, serverURL string) MockRelease {
		// Determine OS/Arch for the asset name
		osName := runtime.GOOS
		arch := runtime.GOARCH
		ext := "tar.gz"
		if osName == "windows" {
			ext = "zip"
		}

		// Asset name uses just the plugin name (e.g., "test_v1.0.0_linux_amd64.tar.gz")
		assetName := fmt.Sprintf("test_%s_%s_%s.%s", tagName, osName, arch, ext)
		downloadPath := fmt.Sprintf("/download/%s", assetName)

		mu.Lock()
		// Create a dummy artifact if it doesn't exist
		if _, exists := artifacts[assetName]; !exists {
			content := createTestArtifactContent(t, "test-plugin", tagName)
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
				w.Write(content)
				return
			}
			http.NotFound(w, r)
			return
		}

		// Handle release metadata requests
		if strings.Contains(r.URL.Path, "/releases/latest") {
			release := createRelease("v1.0.0", "http://"+r.Host)
			json.NewEncoder(w).Encode(release)
			return
		}

		if strings.Contains(r.URL.Path, "/releases/tags/") {
			parts := strings.Split(r.URL.Path, "/")
			tag := parts[len(parts)-1]
			release := createRelease(tag, "http://"+r.Host)
			json.NewEncoder(w).Encode(release)
			return
		}

		http.NotFound(w, r)
	}))

	return server, server.Close
}

// createTestArtifactContent generates a valid zip or tar.gz archive in memory containing a mock binary.
// This is a convenience wrapper around CreateTestPluginArchive using runtime OS/arch.
func createTestArtifactContent(t *testing.T, name, version string) []byte {
	t.Helper()
	return CreateTestPluginArchive(t, name, version, runtime.GOOS, runtime.GOARCH)
}

// CreateTestPluginArchive generates a valid .tar.gz or .zip plugin artifact containing a mock binary.
// This is the main helper function (T004) that generates archives with specified OS/arch.
// For Windows, it creates a .zip file; for other platforms, it creates a .tar.gz file.
func CreateTestPluginArchive(t *testing.T, name, version, targetOS, _ string) []byte {
	t.Helper()

	// Determine binary name based on target OS
	binName := name
	if targetOS == "windows" {
		binName += ".exe"
	}

	// Create mock binary content - a simple script for Unix, placeholder for Windows
	content := []byte("#!/bin/sh\necho 'Mock Plugin " + version + "'")
	if targetOS == "windows" {
		content = []byte("Mock Plugin Executable " + version)
	}

	// Create archive in memory
	var buf bytes.Buffer

	if targetOS == "windows" {
		// Create Zip archive for Windows
		zipWriter := zip.NewWriter(&buf)
		header := &zip.FileHeader{
			Name:   binName,
			Method: zip.Deflate,
		}
		// Set executable permission (Unix mode in extended attributes)
		header.SetMode(0755)
		f, err := zipWriter.CreateHeader(header)
		require.NoError(t, err)
		_, err = f.Write(content)
		require.NoError(t, err)
		require.NoError(t, zipWriter.Close())
	} else {
		// Create Tar.Gz archive for Unix-like systems
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		header := &tar.Header{
			Name: binName,
			Mode: 0755,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write(content)
		require.NoError(t, err)
		require.NoError(t, tw.Close())
		require.NoError(t, gw.Close())
	}

	return buf.Bytes()
}

// installMockPlugin pre-installs a plugin binary for update/remove tests.
// It creates the directory structure and a mock binary file.
func installMockPlugin(t *testing.T, pluginDir, name, version string) {
	t.Helper()
	installDir := filepath.Join(pluginDir, name, version)
	require.NoError(t, os.MkdirAll(installDir, 0755))

	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	err := os.WriteFile(filepath.Join(installDir, binName), []byte("mock binary"), 0755)
	require.NoError(t, err)
}

// MockRegistryConfig configures the mock registry server behavior.
type MockRegistryConfig struct {
	// Plugins maps plugin name to available versions (latest version first)
	Plugins map[string][]string
	// FailDownload if true, returns 500 for download requests
	FailDownload bool
	// FailMetadata if true, returns 500 for release metadata requests
	FailMetadata bool
}

// mockArtifactStore caches generated plugin archives by asset name.
type mockArtifactStore struct {
	mu        sync.Mutex
	artifacts map[string][]byte
}

func newMockArtifactStore() *mockArtifactStore {
	return &mockArtifactStore{artifacts: make(map[string][]byte)}
}

// getOrCreate returns the asset name and size for the plugin release on the
// current platform, generating the archive on first use.
func (s *mockArtifactStore) getOrCreate(t *testing.T, pluginName, tagName string) (string, int64) {
	t.Helper()

	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	assetName := fmt.Sprintf("%s_%s_%s_%s.%s", pluginName, tagName, runtime.GOOS, runtime.GOARCH, ext)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.artifacts[assetName]; !exists {
		s.artifacts[assetName] = CreateTestPluginArchive(t, pluginName, tagName, runtime.GOOS, runtime.GOARCH)
	}
	return assetName, int64(len(s.artifacts[assetName]))
}

// serveDownload writes the artifact for a /download/{filename} request.
func (s *mockArtifactStore) serveDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	s.mu.Lock()
	content, ok := s.artifacts[filename]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// newMockRelease creates a release with a single asset for the current platform.
// The asset name matches what the installer's FindPlatformAssetWithHints looks for.
func newMockRelease(t *testing.T, store *mockArtifactStore, pluginName, tagName, serverURL string) MockRelease {
	t.Helper()

	assetName, size := store.getOrCreate(t, pluginName, tagName)
	return MockRelease{
		TagName: tagName,
		Assets: []MockAsset{
			{
				Name:               assetName,
				BrowserDownloadURL: serverURL + "/download/" + assetName,
				Size:               size,
			},
		},
	}
}

// parsePluginRequest extracts the plugin name from a GitHub-style request path
// (/repos/{owner}/{repo}/releases/...) and resolves its configured versions.
func parsePluginRequest(plugins map[string][]string, path string) (string, []string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return "", nil, false
	}
	// Extract plugin name from repo (e.g., "finfocus-plugin-test" -> "test")
	pluginName := strings.TrimPrefix(parts[2], "finfocus-plugin-")
	versions, exists := plugins[pluginName]
	if !exists || len(versions) == 0 {
		return "", nil, false
	}
	return pluginName, versions, true
}

// releaseTag returns the trailing tag from a /releases/tags/{tag} path.
func releaseTag(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// StartMockRegistryWithConfig creates a configurable mock registry server.
// It allows testing various scenarios like multiple versions, download failures, etc.
func StartMockRegistryWithConfig(t *testing.T, cfg MockRegistryConfig) (*httptest.Server, func()) {
	t.Helper()

	server := httptest.NewServer(&configMockRegistry{t: t, cfg: cfg, store: newMockArtifactStore()})
	return server, server.Close
}

// configMockRegistry serves GitHub-style release and download responses
// according to MockRegistryConfig.
type configMockRegistry struct {
	t     *testing.T
	cfg   MockRegistryConfig
	store *mockArtifactStore
}

func (m *configMockRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/download/") {
		if m.cfg.FailDownload {
			http.Error(w, "simulated download failure", http.StatusInternalServerError)
			return
		}
		m.store.serveDownload(w, r)
		return
	}

	if m.cfg.FailMetadata {
		http.Error(w, "simulated metadata failure", http.StatusInternalServerError)
		return
	}

	pluginName, versions, ok := parsePluginRequest(m.cfg.Plugins, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	m.serveRelease(w, r, pluginName, versions)
}

func (m *configMockRegistry) serveRelease(
	w http.ResponseWriter,
	r *http.Request,
	pluginName string,
	versions []string,
) {
	serverURL := "http://" + r.Host

	switch {
	case strings.Contains(r.URL.Path, "/releases/latest"):
		_ = json.NewEncoder(w).Encode(newMockRelease(m.t, m.store, pluginName, versions[0], serverURL))
	case strings.Contains(r.URL.Path, "/releases/tags/"):
		tag := releaseTag(r.URL.Path)
		if !slices.Contains(versions, tag) {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(newMockRelease(m.t, m.store, pluginName, tag, serverURL))
	default:
		http.NotFound(w, r)
	}
}

// setupTestPluginDir creates a temporary plugin directory for testing.
func setupTestPluginDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
