package registry

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

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
