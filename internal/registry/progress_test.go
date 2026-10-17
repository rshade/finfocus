package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall_ProgressSentencesAndPhases(t *testing.T) {
	t.Parallel()

	pluginName := "aws-public"
	version := "v1.0.0"
	assetName := checksumTestAssetName(pluginName, version)
	archive := createMockArchive(t, pluginName)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/rshade/finfocus-plugin-aws-public/releases/latest":
			release := GitHubRelease{
				TagName: version,
				Name:    version,
				Assets: []ReleaseAsset{{
					Name:               assetName,
					Size:               int64(len(archive)),
					BrowserDownloadURL: "http://" + r.Host + "/download/" + assetName,
				}},
			}
			assert.NoError(t, json.NewEncoder(w).Encode(release))
		case "/download/" + assetName:
			_, err := w.Write(archive)
			assert.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	client := NewGitHubClient()
	client.HTTPClient = server.Client()
	client.BaseURL = server.URL
	pluginDir := filepath.Join(t.TempDir(), "plugins")
	installer := NewInstallerWithClient(client, pluginDir)

	var messages []string
	var events []ProgressEvent
	result, err := installer.Install(context.Background(), pluginName, InstallOptions{
		NoSave: true,
		OnEvent: func(event ProgressEvent) {
			events = append(events, event)
		},
	}, func(msg string) {
		messages = append(messages, msg)
	})
	require.NoError(t, err)
	assert.Equal(t, version, result.Version)

	downloadLine := fmt.Sprintf("Downloading %s (%d bytes)...", assetName, len(archive))
	want := []string{
		"Fetching latest release for aws-public...",
		downloadLine,
		"Downloading... 100%",
		"Warning: checksums.txt not found in release, skipping verification",
		"Extracting archive...",
		"Successfully installed aws-public@v1.0.0",
	}
	assert.Equal(t, want, messages)
	require.Len(t, events, len(messages))

	assert.Equal(t, PhaseResolve, events[0].Phase)
	assert.Equal(t, PhaseDownload, events[1].Phase)
	assert.Equal(t, PhaseDownload, events[2].Phase)
	assert.Equal(t, int64(len(archive)), events[2].Bytes)
	assert.Equal(t, int64(len(archive)), events[2].Total)
	assert.False(t, events[2].Warning)
	assert.Equal(t, PhaseVerify, events[3].Phase)
	assert.True(t, events[3].Warning)
	assert.Equal(t, PhaseExtract, events[4].Phase)
	assert.Equal(t, PhaseDone, events[5].Phase)
	for i := range events {
		assert.Equal(t, messages[i], events[i].Message)
	}

	binary := filepath.Join(pluginDir, pluginName, version, pluginName)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	_, err = os.Stat(binary)
	require.NoError(t, err)
}
