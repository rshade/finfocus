package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// releaseRaceServer serves owner/repo releases where some releases may have no
// assets yet, as happens between Release Please publishing a release and
// GoReleaser uploading its archives. withAssets lists the tags that have assets.
type releaseRaceServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
}

func newReleaseRaceServer(
	t *testing.T,
	repoPath, assetPrefix, binaryName, latest string,
	tags []string,
	withAssets map[string]bool,
	canonical func(tag string) string,
) *releaseRaceServer {
	t.Helper()
	s := &releaseRaceServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.Path)
		s.mu.Unlock()

		release := func(tag string) GitHubRelease {
			rel := GitHubRelease{TagName: tag}
			if withAssets[tag] {
				name := testAssetName(assetPrefix, canonical(tag))
				rel.Assets = []ReleaseAsset{{
					Name:               name,
					BrowserDownloadURL: fmt.Sprintf("%s/download/%s", s.URL, name),
				}}
			}
			return rel
		}

		tagsPath := repoPath + "/releases/tags/"
		switch {
		case r.URL.Path == repoPath+"/releases/latest" && latest != "":
			assert.NoError(t, json.NewEncoder(w).Encode(release(latest)))
		case r.URL.Path == repoPath+"/releases":
			rels := make([]GitHubRelease, 0, len(tags))
			for _, tag := range tags {
				rels = append(rels, release(tag))
			}
			assert.NoError(t, json.NewEncoder(w).Encode(rels))
		case strings.HasPrefix(r.URL.Path, tagsPath):
			tag := strings.TrimPrefix(r.URL.Path, tagsPath)
			for _, known := range tags {
				if known == tag {
					assert.NoError(t, json.NewEncoder(w).Encode(release(tag)))
					return
				}
			}
			http.NotFound(w, r)
		case filepath.Dir(r.URL.Path) == "/download":
			_, _ = w.Write(createMockArchive(t, binaryName))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *releaseRaceServer) requested(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.requests {
		if p == path {
			return true
		}
	}
	return false
}

func newRaceInstaller(t *testing.T, server *httptest.Server) (*Installer, string) {
	t.Helper()
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	return NewInstallerWithClient(client, pluginDir), pluginDir
}

func collectProgress() (func(string), func() []string) {
	var mu sync.Mutex
	var msgs []string
	return func(msg string) {
			mu.Lock()
			defer mu.Unlock()
			msgs = append(msgs, msg)
		}, func() []string {
			mu.Lock()
			defer mu.Unlock()
			return append([]string(nil), msgs...)
		}
}

func identityTag(tag string) string { return tag }

const awsPublicRepoPath = "/repos/rshade/finfocus-plugin-aws-public"

func TestInstall_LatestWithoutAssetsFallsBackToEarlierRelease(t *testing.T) {
	t.Parallel()

	server := newReleaseRaceServer(t, awsPublicRepoPath, "finfocus-plugin-aws-public", "aws-public",
		"v0.2.0",
		[]string{"v0.2.0", "v0.1.1", "v0.1.0"},
		map[string]bool{"v0.1.1": true, "v0.1.0": true},
		identityTag,
	)
	installer, pluginDir := newRaceInstaller(t, server.Server)
	progress, messages := collectProgress()

	result, err := installer.Install(context.Background(), "aws-public", InstallOptions{NoSave: true}, progress)
	require.NoError(t, err)

	assert.Equal(t, "v0.1.1", result.Version)
	assert.Equal(t, filepath.Join(pluginDir, "aws-public", "v0.1.1"), result.Path)

	var warning string
	for _, msg := range messages() {
		if strings.HasPrefix(msg, "Warning:") && strings.Contains(msg, "no assets") {
			warning = msg
		}
	}
	require.NotEmpty(t, warning, "expected a warning about the asset-less release, got %v", messages())
	assert.Contains(t, warning, "v0.2.0")
	assert.Contains(t, warning, "v0.1.1")
}

func TestInstall_PrefixedLatestWithoutAssetsFallsBackToEarlierRelease(t *testing.T) {
	t.Parallel()

	canonical := func(tag string) string { return CanonicalVersion(tag, "kubernetes-") }
	server := newReleaseRaceServer(t, "/repos/rshade/finfocus", "finfocus-plugin-kubernetes",
		"finfocus-plugin-kubernetes",
		"",
		[]string{"kubernetes-v0.3.0", "v0.4.0", "kubernetes-v0.2.0", "kubernetes-v0.1.0"},
		map[string]bool{"v0.4.0": true, "kubernetes-v0.2.0": true, "kubernetes-v0.1.0": true},
		canonical,
	)
	installer, _ := newRaceInstaller(t, server.Server)
	progress, messages := collectProgress()

	result, err := installer.Install(context.Background(), "kubernetes", InstallOptions{NoSave: true}, progress)
	require.NoError(t, err)

	assert.Equal(t, "v0.2.0", result.Version)
	joined := strings.Join(messages(), "\n")
	assert.Contains(t, joined, "v0.3.0")
	assert.Contains(t, joined, "v0.2.0")
}

func TestInstall_LatestWithoutAssetsAndNoEarlierAssetsErrors(t *testing.T) {
	t.Parallel()

	server := newReleaseRaceServer(t, awsPublicRepoPath, "finfocus-plugin-aws-public", "aws-public",
		"v0.2.0",
		[]string{"v0.2.0"},
		map[string]bool{},
		identityTag,
	)
	installer, _ := newRaceInstaller(t, server.Server)

	_, err := installer.Install(context.Background(), "aws-public", InstallOptions{NoSave: true}, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrReleaseHasNoAssets)
	assert.Contains(t, err.Error(), "release v0.2.0 of aws-public has no assets yet")
}

func TestInstall_ExplicitVersionWithoutAssetsDoesNotFallBack(t *testing.T) {
	t.Parallel()

	server := newReleaseRaceServer(t, awsPublicRepoPath, "finfocus-plugin-aws-public", "aws-public",
		"v0.2.0",
		[]string{"v0.2.0", "v0.1.1"},
		map[string]bool{"v0.1.1": true},
		identityTag,
	)
	installer, _ := newRaceInstaller(t, server.Server)

	_, err := installer.Install(context.Background(), "aws-public@v0.2.0", InstallOptions{NoSave: true}, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrReleaseHasNoAssets)
	assert.Contains(t, err.Error(),
		"release v0.2.0 of aws-public has no assets yet (the upload may still be in progress); "+
			"retry shortly or pin an earlier version")
	assert.NotContains(t, err.Error(), "Available: []")
	assert.False(t, server.requested(awsPublicRepoPath+"/releases"), "explicit version must not scan for fallbacks")
}

func TestFetchRelease_LatestWithoutAssetsUsesEarlierRelease(t *testing.T) {
	t.Parallel()

	server := newReleaseRaceServer(t, awsPublicRepoPath, "finfocus-plugin-aws-public", "aws-public",
		"v0.2.0",
		[]string{"v0.2.0", "v0.1.1"},
		map[string]bool{"v0.1.1": true},
		identityTag,
	)
	installer, _ := newRaceInstaller(t, server.Server)
	progress, messages := collectProgress()

	release, err := installer.fetchRelease(
		context.Background(), "rshade", "finfocus-plugin-aws-public", "", nil, progress,
	)
	require.NoError(t, err)
	assert.Equal(t, "v0.1.1", release.TagName)
	assert.Contains(t, strings.Join(messages(), "\n"), "v0.2.0")
}

func TestFindPlatformAssetWithHints_EmptyRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		release     *GitHubRelease
		hints       *AssetNamingHints
		wantNoAsset bool
		wantText    string
	}{
		{
			name:        "zero assets is a distinct error",
			release:     &GitHubRelease{TagName: "v0.2.0"},
			wantNoAsset: true,
			wantText:    "release v0.2.0 of aws-public has no assets yet",
		},
		{
			name:        "zero assets uses canonical version for prefixed tags",
			release:     &GitHubRelease{TagName: "kubernetes-v0.3.0"},
			hints:       &AssetNamingHints{TagPrefix: "kubernetes-"},
			wantNoAsset: true,
			wantText:    "release v0.3.0 of aws-public has no assets yet",
		},
		{
			name: "assets without a platform match keep the existing error",
			release: &GitHubRelease{
				TagName: "v0.2.0",
				Assets:  []ReleaseAsset{{Name: "aws-public_v0.2.0_plan9_mips.tar.gz"}},
			},
			wantText: "no asset found for",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := FindPlatformAssetWithHints(tt.release, "aws-public", tt.hints)
			require.Error(t, err)
			assert.Equal(t, tt.wantNoAsset, errors.Is(err, ErrReleaseHasNoAssets))
			assert.Contains(t, err.Error(), tt.wantText)
		})
	}
}

func TestNewInstaller_DefaultPluginDirFollowsResolvedHome(t *testing.T) {
	tests := []struct {
		name        string
		finfocus    string
		pulumi      string
		wantRelPath string
	}{
		{name: "FINFOCUS_HOME wins", finfocus: "ff", pulumi: "pulumi", wantRelPath: filepath.Join("ff", "plugins")},
		{name: "PULUMI_HOME/finfocus", pulumi: "pulumi", wantRelPath: filepath.Join("pulumi", "finfocus", "plugins")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			ffHome, pulumiHome := "", ""
			if tt.finfocus != "" {
				ffHome = filepath.Join(root, tt.finfocus)
			}
			if tt.pulumi != "" {
				pulumiHome = filepath.Join(root, tt.pulumi)
			}
			t.Setenv("FINFOCUS_HOME", ffHome)
			t.Setenv("PULUMI_HOME", pulumiHome)
			want := filepath.Join(root, tt.wantRelPath)

			assert.Equal(t, want, NewInstaller("").pluginDir)
			assert.Equal(t, want, NewInstallerWithClient(&GitHubClient{}, "").pluginDir)
		})
	}
}

func TestInstall_DefaultPluginDirHonorsFinfocusHome(t *testing.T) {
	ffHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", ffHome)

	server := newReleaseRaceServer(t, awsPublicRepoPath, "finfocus-plugin-aws-public", "aws-public",
		"v0.1.1",
		[]string{"v0.1.1"},
		map[string]bool{"v0.1.1": true},
		identityTag,
	)
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	installer := NewInstallerWithClient(client, "")

	result, err := installer.Install(context.Background(), "aws-public", InstallOptions{NoSave: true}, nil)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(ffHome, "plugins", "aws-public", "v0.1.1"), result.Path)
	assert.DirExists(t, result.Path)
}
