package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGitHubClient(t *testing.T) {
	client := NewGitHubClient()
	if client == nil {
		t.Fatal("NewGitHubClient() returned nil")
	}
	if client.HTTPClient == nil {
		t.Error("HTTPClient is nil")
	}
}

func TestGitHubRelease(t *testing.T) {
	release := GitHubRelease{
		TagName:    "v1.0.0",
		Name:       "Release 1.0.0",
		Draft:      false,
		Prerelease: false,
		Assets: []ReleaseAsset{
			{
				Name:               "plugin-v1.0.0-linux-amd64.tar.gz",
				Size:               1024,
				BrowserDownloadURL: "https://github.com/owner/repo/releases/download/v1.0.0/plugin-v1.0.0-linux-amd64.tar.gz",
				ContentType:        "application/gzip",
			},
		},
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("TagName = %v, want v1.0.0", release.TagName)
	}
	if len(release.Assets) != 1 {
		t.Errorf("len(Assets) = %d, want 1", len(release.Assets))
	}
}

func TestReleaseAsset(t *testing.T) {
	asset := ReleaseAsset{
		Name:               "test-asset.tar.gz",
		Size:               2048,
		BrowserDownloadURL: "https://example.com/asset.tar.gz",
		ContentType:        "application/gzip",
	}

	if asset.Name != "test-asset.tar.gz" {
		t.Errorf("Name = %v, want test-asset.tar.gz", asset.Name)
	}
	if asset.Size != 2048 {
		t.Errorf("Size = %d, want 2048", asset.Size)
	}
}

func TestFindPlatformAsset(t *testing.T) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Determine expected extension
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	// Create release with matching asset
	release := &GitHubRelease{
		TagName: "v1.0.0",
		Assets: []ReleaseAsset{
			{
				Name:               "plugin_v1.0.0_" + goos + "_" + goarch + ext,
				Size:               1024,
				BrowserDownloadURL: "https://example.com/download",
			},
			{
				Name: "plugin_v1.0.0_other_arch.tar.gz",
				Size: 512,
			},
		},
	}

	asset, err := FindPlatformAsset(release, "plugin")
	if err != nil {
		t.Errorf("FindPlatformAsset() error = %v", err)
	}
	if asset == nil {
		t.Fatal("FindPlatformAsset() returned nil asset")
	}
	if asset.Size != 1024 {
		t.Errorf("asset.Size = %d, want 1024", asset.Size)
	}
}

func TestFindPlatformAssetNotFound(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.0.0",
		Assets: []ReleaseAsset{
			{
				Name: "plugin_v1.0.0_unsupported_platform.tar.gz",
				Size: 1024,
			},
		},
	}

	_, err := FindPlatformAsset(release, "plugin")
	if err == nil {
		t.Error("expected error when platform asset not found")
	}
}

func TestFindPlatformAssetEmptyAssets(t *testing.T) {
	release := &GitHubRelease{
		TagName: "v1.0.0",
		Assets:  []ReleaseAsset{},
	}

	_, err := FindPlatformAsset(release, "plugin")
	if err == nil {
		t.Error("expected error when no assets available")
	}
}

func TestGetGitHubToken(t *testing.T) {
	// Test with environment variable set
	t.Setenv("GITHUB_TOKEN", "test-token")
	token := getGitHubToken()
	if token != "test-token" {
		t.Errorf("getGitHubToken() = %v, want test-token", token)
	}

	// Test with empty environment variable (token is optional)
	t.Setenv("GITHUB_TOKEN", "")
	token = getGitHubToken()
	if token != "" {
		t.Errorf("getGitHubToken() with empty env = %v, want empty string", token)
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are defined correctly
	if osWindows != "windows" {
		t.Errorf("osWindows = %v, want windows", osWindows)
	}
	if extExe != ".exe" {
		t.Errorf("extExe = %v, want .exe", extExe)
	}
	if extZip != ".zip" {
		t.Errorf("extZip = %v, want .zip", extZip)
	}
	if extTarGz != ".tar.gz" {
		t.Errorf("extTarGz = %v, want .tar.gz", extTarGz)
	}
}

func TestGitHubClient_ContextCancellation(t *testing.T) {
	// Create a slow server that takes too long to respond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context was cancelled
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"tag_name":"v1.0.0"}`))
		}
	}))
	defer server.Close()

	client := &GitHubClient{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	// Create a context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The request should fail due to context cancellation
	_, err := client.GetLatestRelease(ctx, "owner", "repo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestGitHubClient_ContextCancellation_ListStableReleases(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	client := &GitHubClient{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.ListStableReleases(ctx, "owner", "repo", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestGitHubClient_ContextCancellation_DownloadAsset(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("binary content"))
		}
	}))
	defer server.Close()

	client := &GitHubClient{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	destPath := t.TempDir() + "/asset.bin"
	err := client.DownloadAsset(ctx, server.URL, destPath, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context")
}

func TestGitHubClient_ContextPropagation(t *testing.T) {
	// Verify that context is passed through to HTTP requests
	var requestReceived atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestReceived.Store(true)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.0.0","assets":[]}`))
	}))
	defer server.Close()

	client := &GitHubClient{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	ctx := context.Background()

	_, err := client.GetLatestRelease(ctx, "owner", "repo")
	require.NoError(t, err)
	assert.True(t, requestReceived.Load(), "HTTP request should have been made")
}
