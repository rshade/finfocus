package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectLatestByPrefix(t *testing.T) {
	t.Parallel()

	releases := []GitHubRelease{
		{TagName: "v0.3.8"},
		{TagName: "kubernetes-v0.1.9"}, // published after 0.1.10 (hotfix)
		{TagName: "kubernetes-v0.1.10"},
		{TagName: "prometheus-v0.2.0"},
		{TagName: "kubernetes-vbad"},
	}
	got, err := selectLatestByPrefix(releases, "kubernetes-")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.10", got.TagName)

	_, err = selectLatestByPrefix(releases, "datadog-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tag prefix "datadog-"`)
}

func TestGetLatestReleaseWithPrefix_ScansPastCoreReleases(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, "/repos/rshade/finfocus/releases", r.URL.Path) {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		var rels []GitHubRelease
		for i := 0; i < 20; i++ { // 20 newer CLI releases first
			rels = append(rels, GitHubRelease{TagName: "v0.4." + string(rune('a'+i))})
		}
		rels = append(rels,
			GitHubRelease{TagName: "kubernetes-v0.2.0-rc.1", Prerelease: true},
			GitHubRelease{TagName: "kubernetes-v0.1.0"},
		)
		if !assert.NoError(t, json.NewEncoder(w).Encode(rels)) {
			return
		}
	}))
	defer server.Close()

	c := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	got, err := c.GetLatestReleaseWithPrefix(context.Background(), "rshade", "finfocus", "kubernetes-")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.0", got.TagName, "prerelease must be skipped")
}
