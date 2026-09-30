package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

const (
	jevReleasesPath = "/repos/rshade/finfocus/releases"
	jevTagsPath     = "/repos/rshade/finfocus/releases/tags/"
)

type jevReleaseServer struct {
	*httptest.Server

	mu   sync.Mutex
	tags []string
}

func (s *jevReleaseServer) publish(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tags = append(s.tags, tag)
}

func (s *jevReleaseServer) release(tag string) GitHubRelease {
	name := testAssetName("finfocus-plugin-jev", CanonicalVersion(tag, "jev-"))
	return GitHubRelease{TagName: tag, Assets: []ReleaseAsset{{
		Name:               name,
		BrowserDownloadURL: fmt.Sprintf("%s/download/%s", s.URL, name),
	}}}
}

func (s *jevReleaseServer) writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func (s *jevReleaseServer) handle(t *testing.T, w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	tags := append([]string(nil), s.tags...)
	s.mu.Unlock()

	switch {
	case r.URL.Path == jevReleasesPath:
		releases := []GitHubRelease{{TagName: "v9.9.9"}}
		for _, tag := range tags {
			releases = append(releases, s.release(tag))
		}
		s.writeJSON(t, w, releases)
	case strings.HasPrefix(r.URL.Path, jevTagsPath):
		want := strings.TrimPrefix(r.URL.Path, jevTagsPath)
		for _, tag := range tags {
			if tag == want {
				s.writeJSON(t, w, s.release(tag))
				return
			}
		}
		http.NotFound(w, r)
	case filepath.Dir(r.URL.Path) == "/download":
		_, _ = w.Write(createMockArchive(t, "finfocus-plugin-jev"))
	default:
		t.Errorf("unexpected request %s", r.URL.Path)
		http.NotFound(w, r)
	}
}

func newJevUpdateFixture(t *testing.T, tags ...string) (*jevReleaseServer, *Installer, string) {
	t.Helper()
	srv := &jevReleaseServer{tags: tags}
	srv.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.handle(t, w, r)
	}))
	t.Cleanup(srv.Close)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FINFOCUS_HOME", filepath.Join(home, ".finfocus"))
	config.ResetGlobalConfigForTest()
	config.InitGlobalConfig()

	pluginDir := filepath.Join(home, ".finfocus", "plugins")
	client := &GitHubClient{HTTPClient: srv.Client(), BaseURL: srv.URL}
	return srv, NewInstallerWithClient(client, pluginDir), pluginDir
}

func installJev(t *testing.T, inst *Installer, version string) {
	t.Helper()
	spec := "jev"
	if version != "" {
		spec += "@" + version
	}
	res, err := inst.Install(context.Background(), spec, InstallOptions{}, nil)
	require.NoError(t, err)
	require.Equal(t, "v0.1.0", res.Version)
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via newJevUpdateFixture)
func TestUpdate_JevRegistryEntry_UpdatesToNewerPrefixedTag(t *testing.T) {
	srv, inst, pluginDir := newJevUpdateFixture(t, "jev-v0.1.0")
	installJev(t, inst, "v0.1.0")

	srv.publish("jev-v0.2.0")
	res, err := inst.Update(context.Background(), "jev", UpdateOptions{}, nil)
	require.NoError(t, err)

	assert.False(t, res.WasUpToDate)
	assert.Equal(t, "v0.1.0", res.OldVersion)
	assert.Equal(t, "v0.2.0", res.NewVersion)
	assert.DirExists(t, filepath.Join(pluginDir, "jev", "v0.2.0"))
	_, statErr := os.Stat(filepath.Join(pluginDir, "jev", "v0.1.0"))
	assert.True(t, os.IsNotExist(statErr), "old version directory must be removed")

	installed, err := config.GetInstalledPlugin("jev")
	require.NoError(t, err)
	assert.Equal(t, "v0.2.0", installed.Version)
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via newJevUpdateFixture)
func TestUpdate_JevRegistryEntry_NoOpWhenCurrent(t *testing.T) {
	_, inst, pluginDir := newJevUpdateFixture(t, "jev-v0.1.0")
	installJev(t, inst, "")

	res, err := inst.Update(context.Background(), "jev", UpdateOptions{}, nil)
	require.NoError(t, err)

	assert.True(t, res.WasUpToDate)
	assert.Equal(t, "v0.1.0", res.OldVersion)
	assert.Equal(t, "v0.1.0", res.NewVersion)
	assert.DirExists(t, filepath.Join(pluginDir, "jev", "v0.1.0"))
}

//nolint:paralleltest // t.Setenv changes the process-wide environment (via newJevUpdateFixture)
func TestUpdate_JevRegistryEntry_IgnoresCoreReleaseTags(t *testing.T) {
	srv, inst, _ := newJevUpdateFixture(t, "jev-v0.1.0")
	installJev(t, inst, "v0.1.0")
	srv.publish("kubernetes-v5.0.0")

	res, err := inst.Update(context.Background(), "jev", UpdateOptions{}, nil)
	require.NoError(t, err)

	assert.True(t, res.WasUpToDate, "v9.9.9 core tag and other plugins' tags must not trigger an update")
}
