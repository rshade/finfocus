package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGetLatestRelease(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" {
			t.Errorf("Expected path /repos/owner/repo/releases/latest, got %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		release := GitHubRelease{
			TagName: "v1.0.0",
			Name:    "Release 1.0.0",
			Assets:  []ReleaseAsset{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// Configure client to use mock server
	// Note: Since GetLatestRelease constructs the URL using "https://api.github.com",
	// we can't easily redirect it to our mock server unless we override the base URL.
	// However, fetchRelease takes a full URL.
	// So we should test fetchRelease directly or refactor GetLatestRelease to allow base URL override.

	// For now, let's test fetchRelease directly as it is the core logic.
	client := NewGitHubClient()
	client.HTTPClient = server.Client()

	release, err := client.fetchRelease(server.URL + "/repos/owner/repo/releases/latest")
	if err != nil {
		t.Fatalf("fetchRelease failed: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected tag v1.0.0, got %s", release.TagName)
	}
}

func TestGetReleaseByTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/tags/v1.0.0" {
			http.NotFound(w, r)
			return
		}

		release := GitHubRelease{
			TagName: "v1.0.0",
		}
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.HTTPClient = server.Client()

	// Testing fetchRelease with constructed URL from test
	release, err := client.fetchRelease(server.URL + "/repos/owner/repo/releases/tags/v1.0.0")
	if err != nil {
		t.Fatalf("fetchRelease failed: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected tag v1.0.0, got %s", release.TagName)
	}
}

func TestDownloadAsset(t *testing.T) {
	content := "binary content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		w.Write([]byte(content))
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.HTTPClient = server.Client()

	tmpDir := t.TempDir()
	destPath := filepath.Join(tmpDir, "asset.bin")

	err := client.DownloadAsset(server.URL, destPath, nil)
	if err != nil {
		t.Fatalf("DownloadAsset failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestFetchRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.HTTPClient = server.Client()

	_, err := client.fetchRelease(server.URL)
	if err == nil {
		t.Error("Expected error for 404, got nil")
	}
}

func TestFetchRelease_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.HTTPClient = server.Client()

	_, err := client.fetchRelease(server.URL)
	if err == nil {
		t.Error("Expected error for 403, got nil")
	}
}

func TestListStableReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases" {
			http.NotFound(w, r)
			return
		}

		releases := []GitHubRelease{
			{TagName: "v2.0.0", Name: "Release 2.0.0", Draft: false, Prerelease: false},
			{TagName: "v2.0.0-beta", Name: "Beta", Draft: false, Prerelease: true},
			{TagName: "v1.5.0", Name: "Release 1.5.0", Draft: false, Prerelease: false},
			{TagName: "v1.4.0-draft", Name: "Draft", Draft: true, Prerelease: false},
			{TagName: "v1.0.0", Name: "Release 1.0.0", Draft: false, Prerelease: false},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("Failed to encode releases: %v", err)
		}
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	releases, err := client.ListStableReleases("owner", "repo", 10)
	if err != nil {
		t.Fatalf("ListStableReleases failed: %v", err)
	}

	// Should only return stable releases (not draft, not prerelease)
	if len(releases) != 3 {
		t.Errorf("Expected 3 stable releases, got %d", len(releases))
	}

	expectedTags := []string{"v2.0.0", "v1.5.0", "v1.0.0"}
	for i, expected := range expectedTags {
		if releases[i].TagName != expected {
			t.Errorf("Expected release[%d].TagName = %s, got %s", i, expected, releases[i].TagName)
		}
	}
}

func TestListStableReleases_WithLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []GitHubRelease{
			{TagName: "v3.0.0", Draft: false, Prerelease: false},
			{TagName: "v2.0.0", Draft: false, Prerelease: false},
			{TagName: "v1.0.0", Draft: false, Prerelease: false},
		}
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("Failed to encode releases: %v", err)
		}
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	releases, err := client.ListStableReleases("owner", "repo", 2)
	if err != nil {
		t.Fatalf("ListStableReleases failed: %v", err)
	}

	// Should respect the limit
	if len(releases) != 2 {
		t.Errorf("Expected 2 releases (limit), got %d", len(releases))
	}
}

func TestListStableReleases_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	_, err := client.ListStableReleases("owner", "repo", 10)
	if err == nil {
		t.Error("Expected error for 404, got nil")
	}
}

func TestFindReleaseWithAsset_ExactVersionFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v1.0.0":
			release := GitHubRelease{
				TagName: "v1.0.0",
				Assets: []ReleaseAsset{
					{Name: "plugin_v1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "http://dl/1"},
				},
			}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Errorf("Failed to encode release: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	release, asset, err := client.FindReleaseWithAsset("owner", "repo", "v1.0.0", "plugin", nil)
	if err != nil {
		t.Fatalf("FindReleaseWithAsset failed: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected release v1.0.0, got %s", release.TagName)
	}
	if asset == nil {
		t.Fatal("Expected asset, got nil")
	}
}

func TestFindReleaseWithAsset_FallbackToStable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v2.0.0":
			// Version exists but has no matching asset
			release := GitHubRelease{
				TagName: "v2.0.0",
				Assets: []ReleaseAsset{
					{Name: "plugin_v2.0.0_windows_amd64.zip", BrowserDownloadURL: "http://dl/win"},
				},
			}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Errorf("Failed to encode release: %v", err)
			}
		case "/repos/owner/repo/releases":
			// Fallback releases - v1.0.0 has Linux asset
			releases := []GitHubRelease{
				{
					TagName: "v2.0.0",
					Assets: []ReleaseAsset{
						{Name: "plugin_v2.0.0_windows_amd64.zip", BrowserDownloadURL: "http://dl/win"},
					},
				},
				{
					TagName: "v1.0.0",
					Assets: []ReleaseAsset{
						{Name: "plugin_v1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "http://dl/linux"},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(releases); err != nil {
				t.Errorf("Failed to encode releases: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	// Request v2.0.0 which doesn't have Linux asset, should fallback to v1.0.0
	release, asset, err := client.FindReleaseWithAsset("owner", "repo", "v2.0.0", "plugin", nil)
	if err != nil {
		t.Fatalf("FindReleaseWithAsset failed: %v", err)
	}

	// Should have fallen back to v1.0.0 which has the Linux asset
	if release.TagName != "v1.0.0" {
		t.Errorf("Expected fallback to v1.0.0, got %s", release.TagName)
	}
	if asset == nil || asset.Name != "plugin_v1.0.0_linux_amd64.tar.gz" {
		t.Errorf("Expected Linux asset, got %v", asset)
	}
}

func TestFindReleaseWithAsset_NoVersionSpecified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/owner/repo/releases" {
			releases := []GitHubRelease{
				{
					TagName: "v1.0.0",
					Assets: []ReleaseAsset{
						{Name: "plugin_v1.0.0_linux_amd64.tar.gz", BrowserDownloadURL: "http://dl/1"},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(releases); err != nil {
				t.Errorf("Failed to encode releases: %v", err)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	// Empty version should search stable releases
	release, asset, err := client.FindReleaseWithAsset("owner", "repo", "", "plugin", nil)
	if err != nil {
		t.Fatalf("FindReleaseWithAsset failed: %v", err)
	}

	if release.TagName != "v1.0.0" {
		t.Errorf("Expected v1.0.0, got %s", release.TagName)
	}
	if asset == nil {
		t.Fatal("Expected asset, got nil")
	}
}

func TestFindReleaseWithAsset_NoCompatibleAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/releases/tags/v1.0.0":
			release := GitHubRelease{
				TagName: "v1.0.0",
				Assets: []ReleaseAsset{
					{Name: "plugin_v1.0.0_freebsd_amd64.tar.gz", BrowserDownloadURL: "http://dl/1"},
				},
			}
			if err := json.NewEncoder(w).Encode(release); err != nil {
				t.Errorf("Failed to encode release: %v", err)
			}
		case "/repos/owner/repo/releases":
			// All releases have incompatible assets
			releases := []GitHubRelease{
				{
					TagName: "v1.0.0",
					Assets: []ReleaseAsset{
						{Name: "plugin_v1.0.0_freebsd_amd64.tar.gz", BrowserDownloadURL: "http://dl/1"},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(releases); err != nil {
				t.Errorf("Failed to encode releases: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()

	_, _, err := client.FindReleaseWithAsset("owner", "repo", "v1.0.0", "plugin", nil)
	if err == nil {
		t.Error("Expected error for no compatible asset, got nil")
	}
}
