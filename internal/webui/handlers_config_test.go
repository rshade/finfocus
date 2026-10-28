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

func TestConfigAPIClassifiesWriteErrors(t *testing.T) {
	t.Parallel()
	const leakedPath = "/home/someone/.finfocus/config.hujson"
	tests := []struct {
		name    string
		err     error
		status  int
		code    string
		visible string
		hidden  string
	}{
		{
			name:    "invalid change",
			err:     &config.InvalidChangeError{Message: "unknown configuration section: nope"},
			status:  http.StatusBadRequest,
			code:    "invalid_request",
			visible: "unknown configuration section: nope",
		},
		{
			name:   "file system failure",
			err:    &os.PathError{Op: "open", Path: leakedPath, Err: os.ErrPermission},
			status: http.StatusInternalServerError,
			code:   "internal",
			hidden: leakedPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(context.Background(), SessionOptions{
				ConfigPreview: func(context.Context, config.WriteRequest) (config.PatchResult, error) {
					return config.PatchResult{}, tt.err
				},
				ConfigSave: func(context.Context, config.WriteRequest) (config.PatchResult, error) {
					return config.PatchResult{}, tt.err
				},
			})
			server := newRunningServer(t, Options{Session: session})
			body := `{"tier": "global", "baseVersion": "absent", "changes": []}`
			for _, path := range []string{"/api/config/preview", "/api/config"} {
				response := overviewRequest(t, server, http.MethodPost, path, body)
				text := readBody(t, response)
				assert.Equal(t, tt.status, response.StatusCode, path)
				assert.Contains(t, text, `"code":"`+tt.code+`"`, path)
				if tt.visible != "" {
					assert.Contains(t, text, tt.visible, path)
				}
				if tt.hidden != "" {
					assert.NotContains(t, text, tt.hidden, path)
				}
			}
		})
	}
}

func TestConfigAPIPulumiCloudHistory(t *testing.T) {
	t.Parallel()
	global := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(global, []byte(`{}`), 0o600))
	session := NewSession(context.Background(), SessionOptions{
		ConfigView: func(context.Context) (config.Page, error) { return config.BuildPage(global, "") },
		ConfigPreview: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, true)
		},
		ConfigSave: func(_ context.Context, request config.WriteRequest) (config.PatchResult, error) {
			return writeConfig(global, request, false)
		},
	})
	server := newRunningServer(t, Options{Session: session})
	response := overviewRequest(t, server, http.MethodGet, "/api/config", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	var page struct {
		Versions map[string]string `json:"versions"`
		Settings []struct {
			Key       string   `json:"key"`
			Kind      string   `json:"kind"`
			Enum      []string `json:"enum"`
			Effective struct {
				Source string `json:"source"`
				Value  any    `json:"value"`
			} `json:"effective"`
		} `json:"settings"`
	}
	require.NoError(t, json.Unmarshal([]byte(readBody(t, response)), &page))
	found := false
	for _, row := range page.Settings {
		if row.Key != "cost.history.pulumi_cloud.enabled" {
			continue
		}
		found = true
		assert.Equal(t, "enum", row.Kind)
		assert.Equal(t, []string{"false", "auto", "true"}, row.Enum)
		assert.Equal(t, "false", row.Effective.Value)
		assert.Equal(t, config.SourceDefault, row.Effective.Source)
	}
	require.True(t, found, "Cloud history setting must be served by shared Config view")
	requestBody := func(value string) string {
		return `{"tier":"global","baseVersion":"` + page.Versions[config.TierGlobal] + `","changes":[{"key":"cost.history.pulumi_cloud.enabled","value":"` + value + `"}]}`
	}
	preview := overviewRequest(
		t,
		server,
		http.MethodPost,
		"/api/config/preview",
		requestBody("auto"),
	)
	previewBody := readBody(t, preview)
	require.Equal(t, http.StatusOK, preview.StatusCode, previewBody)
	assert.Contains(t, previewBody, `"after":"\"auto\""`)
	data, err := os.ReadFile(global)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(data))
	for _, path := range []string{"/api/config/preview", "/api/config"} {
		rejected := overviewRequest(t, server, http.MethodPost, path, requestBody("maybe"))
		assert.Equal(t, http.StatusBadRequest, rejected.StatusCode)
		assert.Contains(t, readBody(t, rejected), `"code":"validation"`)
	}
	saved := overviewRequest(t, server, http.MethodPost, "/api/config", requestBody("auto"))
	savedBody := readBody(t, saved)
	require.Equal(t, http.StatusOK, saved.StatusCode, savedBody)
	data, err = os.ReadFile(global)
	require.NoError(t, err)
	assert.JSONEq(t, `{"cost":{"history":{"pulumi_cloud":{"enabled":"auto"}}}}`, string(data))
}
