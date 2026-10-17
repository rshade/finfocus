package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestConfigAPIRedactsAndRejects(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	const secret = "stored-secret-value"
	require.NoError(t, os.WriteFile(global, []byte(`{
	  "output": {"precision": 2},
	  "plugins": {"aws": {"region": "us-east-1", "secret_key": "`+secret+`"}},
	  "cost": {"budgets": {"global": {"alerts": [{"threshold": 80, "type": "actual", "notifications": [
	    {"type": "webhook", "url": "https://hooks.example/notify-secret"}
	  ]}]}}}
	}`), 0o600))
	var logs bytes.Buffer
	logger := zerolog.New(&logs)
	session := NewSession(context.Background(), SessionOptions{
		ConfigView: func(context.Context) (config.Page, error) {
			return config.BuildPage(global, "")
		},
		ConfigPreview: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, true)
		},
		ConfigSave: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, false)
		},
	})
	server := newRunningServer(t, Options{Session: session, Logger: &logger})

	response := overviewRequest(t, server, http.MethodGet, "/api/config", "")
	body := readBody(t, response)
	require.Equal(t, http.StatusOK, response.StatusCode)
	assert.NotContains(t, body, secret)
	assert.NotContains(t, body, "notify-secret")
	assert.Contains(t, body, "us-east-1")
	assert.Contains(t, body, `"projectAvailable":false`)

	var page config.Page
	require.NoError(t, json.Unmarshal([]byte(body), &page))
	preview := overviewRequest(t, server, http.MethodPost, "/api/config/preview", `{
	  "tier": "global",
	  "baseVersion": "`+page.Versions[config.TierGlobal]+`",
	  "changes": [{"key": "output.precision", "value": "3"}]
	}`)
	previewBody := readBody(t, preview)
	require.Equal(t, http.StatusOK, preview.StatusCode, previewBody)
	assert.Contains(t, previewBody, `"after":"3"`)
	onDisk, err := os.ReadFile(global)
	require.NoError(t, err)
	assert.Contains(t, string(onDisk), secret)
	assert.NotContains(t, string(onDisk), `"precision": 3`)

	stale := overviewRequest(t, server, http.MethodPost, "/api/config", `{
	  "tier": "global", "baseVersion": "stale",
	  "changes": [{"key": "output.precision", "value": "3"}]
	}`)
	staleBody := readBody(t, stale)
	assert.Equal(t, http.StatusConflict, stale.StatusCode)
	assert.Contains(t, staleBody, `"code":"conflict"`)

	missingProject := overviewRequest(t, server, http.MethodPost, "/api/config", `{
	  "tier": "project", "baseVersion": "absent",
	  "changes": [{"key": "output.precision", "value": "3"}]
	}`)
	projectBody := readBody(t, missingProject)
	assert.Equal(t, http.StatusBadRequest, missingProject.StatusCode)
	assert.Contains(t, projectBody, `"code":"project_unavailable"`)

	unknown := overviewRequest(t, server, http.MethodPost, "/api/config/preview", `{
	  "tier": "global", "baseVersion": "absent", "changes": [], "path": "/tmp/evil"
	}`)
	assert.Equal(t, http.StatusBadRequest, unknown.StatusCode)

	noOrigin := configRequest(t, server, http.MethodPost, "/api/config", `{}`, false, "application/json")
	assert.Equal(t, http.StatusForbidden, noOrigin.StatusCode)
	plain := configRequest(t, server, http.MethodPost, "/api/config", `{}`, true, "text/plain")
	assert.Equal(t, http.StatusUnsupportedMediaType, plain.StatusCode)

	saved := overviewRequest(t, server, http.MethodPost, "/api/config", `{
	  "tier": "global",
	  "baseVersion": "`+page.Versions[config.TierGlobal]+`",
	  "changes": [{"key": "plugins.aws.secret_key", "value": "replacement-secret"}]
	}`)
	savedBody := readBody(t, saved)
	require.Equal(t, http.StatusOK, saved.StatusCode, savedBody)
	assert.Contains(t, savedBody, "does not reload")
	assert.NotContains(t, savedBody, secret)
	assert.NotContains(t, savedBody, "replacement-secret")
	assert.NotContains(t, logs.String(), secret)
	assert.NotContains(t, logs.String(), "replacement-secret")
	rewritten, err := os.ReadFile(global)
	require.NoError(t, err)
	assert.Contains(t, string(rewritten), "replacement-secret")
}

func writeConfig(global string, request config.WriteRequest, dryRun bool) (config.PatchResult, error) {
	path, err := config.TierPath(global, "", request.Tier)
	if err != nil {
		return config.PatchResult{}, err
	}
	return config.PatchFile(path, request.Tier, request.BaseVersion, request.Changes, dryRun)
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(data)
}

func configRequest(
	t *testing.T,
	server *Server,
	method, path, body string,
	origin bool,
	contentType string,
) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, server.BaseURL()+path, strings.NewReader(body))
	require.NoError(t, err)
	request.AddCookie(&http.Cookie{Name: server.cookieName(), Value: server.token})
	if origin {
		request.Header.Set("Origin", server.BaseURL())
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, response.Body.Close()) })
	return response
}

func TestConfigAPIRequiresBaseVersion(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	original := []byte(`{"output": {"precision": 2}}`)
	require.NoError(t, os.WriteFile(global, original, 0o600))
	session := NewSession(context.Background(), SessionOptions{
		ConfigPreview: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, true)
		},
		ConfigSave: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, false)
		},
	})
	server := newRunningServer(t, Options{Session: session})
	change := `"changes": [{"key": "output.precision", "value": "3"}]`
	bodies := map[string]string{
		"omitted": `{"tier": "global", ` + change + `}`,
		"empty":   `{"tier": "global", "baseVersion": "", ` + change + `}`,
		"null":    `{"tier": "global", "baseVersion": null, ` + change + `}`,
	}
	for name, body := range bodies {
		for _, path := range []string{"/api/config/preview", "/api/config"} {
			response := overviewRequest(t, server, http.MethodPost, path, body)
			assert.Equal(t, http.StatusBadRequest, response.StatusCode, "%s %s", name, path)
			assert.Contains(t, readBody(t, response), "baseVersion")
		}
	}
	onDisk, err := os.ReadFile(global)
	require.NoError(t, err)
	assert.Equal(t, string(original), string(onDisk))
}
