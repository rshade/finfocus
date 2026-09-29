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

// StartMockRegistryWithConfig creates a configurable mock registry server.
// It allows testing various scenarios like multiple versions, download failures, etc.
func StartMockRegistryWithConfig(t *testing.T, cfg MockRegistryConfig) (*httptest.Server, func()) {
	t.Helper()

	// Track created artifacts to serve them later
	var mu sync.Mutex
	artifacts := make(map[string][]byte)

	// Helper to create a release response for a specific plugin/version
	createReleaseForPlugin := func(pluginName, tagName, serverURL string) MockRelease {
		osName := runtime.GOOS
		arch := runtime.GOARCH
		ext := "tar.gz"
		if osName == "windows" {
			ext = "zip"
		}

		// Asset name uses just the plugin name (e.g., "test_v1.0.0_linux_amd64.tar.gz")
		// This matches what the installer's FindPlatformAssetWithHints looks for
		assetName := fmt.Sprintf("%s_%s_%s_%s.%s", pluginName, tagName, osName, arch, ext)
		downloadPath := fmt.Sprintf("/download/%s", assetName)

		mu.Lock()
		// Create artifact if it doesn't exist
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle download requests
		if strings.HasPrefix(r.URL.Path, "/download/") {
			if cfg.FailDownload {
				http.Error(w, "simulated download failure", http.StatusInternalServerError)
				return
			}
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

		if cfg.FailMetadata {
			http.Error(w, "simulated metadata failure", http.StatusInternalServerError)
			return
		}

		// Parse owner/repo from path: /repos/{owner}/{repo}/releases/...
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 4 {
			http.NotFound(w, r)
			return
		}

		// Extract plugin name from repo (e.g., "finfocus-plugin-test" -> "test")
		repo := parts[2]
		pluginName := strings.TrimPrefix(repo, "finfocus-plugin-")

		// Get versions for this plugin
		versions, exists := cfg.Plugins[pluginName]
		if !exists || len(versions) == 0 {
			http.NotFound(w, r)
			return
		}

		// Handle release metadata requests
		if strings.Contains(r.URL.Path, "/releases/latest") {
			release := createReleaseForPlugin(pluginName, versions[0], "http://"+r.Host)
			_ = json.NewEncoder(w).Encode(release)
			return
		}

		if strings.Contains(r.URL.Path, "/releases/tags/") {
			tag := parts[len(parts)-1]
			// Verify this version exists
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

// setupTestPluginDir creates a temporary plugin directory for testing.
func setupTestPluginDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
