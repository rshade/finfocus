package webui

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginRegistryEndpoints(t *testing.T) {
	t.Parallel()
	const sourceURL = "https://registry.example/index.json?token=private-value"
	added := make(chan PluginRegistryRequest, 1)
	removed := make(chan string, 1)
	session := NewSession(context.Background(), SessionOptions{
		PluginRegistries: func(context.Context) ([]PluginRegistryRow, error) {
			return []PluginRegistryRow{{Name: "local", URL: sourceURL, Trust: "community"}}, nil
		},
		PluginRegistryAdd: func(_ context.Context, request PluginRegistryRequest) error {
			added <- request
			return nil
		},
		PluginRegistryRemove: func(_ context.Context, name string) error {
			removed <- name
			return nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	list := pluginRequest(t, server, http.MethodGet, "/api/plugins/registries", "")
	require.Equal(t, http.StatusOK, list.StatusCode)
	assert.JSONEq(
		t,
		`{"registries":[{"name":"local","url":"https://registry.example/index.json","trust":"community"}]}`,
		readBody(t, list),
	)

	req, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		server.BaseURL()+"/api/plugins/registries",
		strings.NewReader(`{"name":"local","url":"https://registry.example"}`),
	)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: server.cookieName(), Value: server.token})
	req.Header.Set("Content-Type", "application/json")
	denied, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer denied.Body.Close()
	assert.Equal(t, http.StatusForbidden, denied.StatusCode)
	assert.Empty(t, added)

	created := pluginRequest(t, server, http.MethodPost, "/api/plugins/registries",
		`{"name":"local","url":"https://registry.example/index.json?token=private-value","trust":"experimental"}`)
	require.Equal(t, http.StatusNoContent, created.StatusCode)
	assert.Equal(t, PluginRegistryRequest{Name: "local", URL: sourceURL, Trust: "experimental"}, <-added)
	deleted := pluginRequest(t, server, http.MethodDelete, "/api/plugin-registries/local", "")
	require.Equal(t, http.StatusNoContent, deleted.StatusCode)
	assert.Equal(t, "local", <-removed)
}

func TestCustomPluginConfirmationCannotBypassTrustOrChecksum(t *testing.T) {
	t.Parallel()
	installed := make(chan PluginOperationRequest, 1)
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(_ context.Context, name string, request PluginPlanRequest) (PluginPlan, error) {
			assert.Equal(t, "work", request.Registry)
			checksum := "present"
			if request.Version == "missing" {
				checksum = "missing"
			}
			return PluginPlan{
				Action:                       request.Action,
				Name:                         name,
				Version:                      request.Version,
				Repository:                   "acme/example",
				SecurityLevel:                "community",
				ChecksumFile:                 checksum,
				InCatalog:                    true,
				Registry:                     request.Registry,
				RegistryRevision:             "current",
				RegistryConfirmationRequired: true,
			}, nil
		},
		PluginInstall: func(_ context.Context, _ string, request PluginOperationRequest, _ func(PluginProgress)) error {
			installed <- request
			return nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	for _, body := range []string{
		`{"version":"v1","repository":"acme/example","securityLevel":"community","registry":"work"}`,
		`{"version":"v1","repository":"acme/example","securityLevel":"community","registry":"work","registryConfirmation":"work","registryRevision":"stale"}`,
		`{"version":"v1","repository":"acme/example","securityLevel":"community","registry":"work","registryConfirmation":"WORK","registryRevision":"current"}`,
		`{"version":"missing","repository":"acme/example","securityLevel":"community","registry":"work","registryConfirmation":"work","registryRevision":"current","acceptMissingChecksum":true}`,
	} {
		response := pluginRequest(t, server, http.MethodPost, "/api/plugins/example/install", body)
		assert.Equal(t, http.StatusConflict, response.StatusCode)
		assert.Empty(t, installed)
	}
	ok := pluginRequest(
		t,
		server,
		http.MethodPost,
		"/api/plugins/example/install",
		`{"version":"v1","repository":"acme/example","securityLevel":"community","registry":"work","registryConfirmation":"work","registryRevision":"current"}`,
	)
	require.Equal(t, http.StatusAccepted, ok.StatusCode)
	select {
	case request := <-installed:
		assert.Equal(t, "work", request.RegistryConfirmation)
		assert.Equal(t, "current", request.RegistryRevision)
		assert.True(t, request.RequireChecksumFile)
	case <-time.After(time.Second):
		t.Fatal("plugin installation did not start")
	}
}
