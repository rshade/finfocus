package registry_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
)

const sampleCustomIndex = `{
	"schema_version": "1.0.0",
	"plugins": {
		"z-plugin": {
			"name": "z-plugin", "description": "Example plugin", "repository": "example/z-plugin",
			"author": "Example", "supported_providers": ["aws"],
			"capabilities": ["actual_costs"], "security_level": "official", "min_spec_version": "0.7.0",
			"max_spec_version": "0.7.12", "keywords": ["cost"]
		},
		"a-plugin": {
			"name": "a-plugin", "description": "Second example", "repository": "example/a.plugin",
			"author": "Example", "supported_providers": ["gcp"],
			"security_level": "untrusted", "min_spec_version": "0.7.0",
			"deprecated": true, "deprecation_message": "Use z-plugin."
		}
	}
}`

func TestCustomIndexFetchRevalidatesETag(t *testing.T) {
	t.Parallel()
	const etag = `"index-v1"`
	var hits atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = io.WriteString(w, sampleCustomIndex)
	}))
	t.Cleanup(server.Close)
	client := registry.NewCustomIndexClient(server.Client())
	src := config.PluginRegistrySource{Name: "example", URL: server.URL}

	entries, err := client.Fetch(t.Context(), src)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "a-plugin", entries[0].Name)
	assert.Equal(t, "experimental", entries[0].SecurityLevel)
	assert.True(t, entries[0].Deprecated)
	assert.Equal(t, "Use z-plugin.", entries[0].DeprecationNotice())
	assert.Equal(t, "z-plugin", entries[1].Name)
	assert.Equal(t, "community", entries[1].SecurityLevel)
	assert.Equal(t, "0.7.12", entries[1].MaxSpecVersion)
	assert.Equal(t, []string{"cost"}, entries[1].Keywords)
	entries[1].SupportedProviders[0] = "changed"
	entries[1].Keywords[0] = "changed"

	cached, err := client.Fetch(t.Context(), src)
	require.NoError(t, err)
	require.Len(t, cached, 2)
	assert.EqualValues(t, 2, hits.Load())
	assert.Equal(t, []string{"aws"}, cached[1].SupportedProviders)
	assert.Equal(t, []string{"cost"}, cached[1].Keywords)

	src.Trust = "experimental"
	entries, err = client.Fetch(t.Context(), src)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "experimental", entries[1].SecurityLevel)
}

func TestCustomIndexValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		body    string
		pin     string
		wantErr string
	}{
		{"valid pin", sampleCustomIndex, fmt.Sprintf("%x", sha256.Sum256([]byte(sampleCustomIndex))), ""},
		{"bad pin", sampleCustomIndex, strings.Repeat("0", 64), "checksum"},
		{"malformed", "not json", "", "invalid JSON"},
		{"unsupported fields", strings.Replace(sampleCustomIndex, `"keywords"`, `"secret-value"`, 1), "", "schema"},
		{"empty plugins", `{"schema_version":"1.0.0","plugins":null}`, "", "schema"},
		{
			"entry mismatch",
			strings.Replace(sampleCustomIndex, `"name": "z-plugin"`, `"name": "zz-plugin"`, 1),
			"",
			"name",
		},
		{"unsafe repo", strings.Replace(sampleCustomIndex, "example/a.plugin", "example/..", 1), "", "repository"},
		{"missing required fields", `{"schema_version":"1.0.0","plugins":{"sample":{"name":"sample"}}}`, "", "schema"},
		{"duplicate keys", `{"schema_version":"1.0.0","plugins":{},"plugins":{}}`, "", "invalid JSON"},
		{"expanded body", strings.Repeat(" ", 1<<21), "", "size limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			t.Cleanup(server.Close)
			src := config.PluginRegistrySource{Name: "example", URL: server.URL, SHA256: tt.pin}
			_, err := registry.NewCustomIndexClient(server.Client()).Fetch(t.Context(), src)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
			assert.NotContains(t, err.Error(), server.URL)
			assert.NotContains(t, err.Error(), "secret-value")
		})
	}
}

func TestCustomIndexRedirectPolicy(t *testing.T) {
	t.Parallel()
	var externalHits atomic.Int32
	external := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		externalHits.Add(1)
		_, _ = io.WriteString(w, sampleCustomIndex)
	}))
	t.Cleanup(external.Close)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/same-origin":
			http.Redirect(w, r, "/index", http.StatusFound)
		case "/external":
			http.Redirect(w, r, external.URL, http.StatusFound)
		case "/insecure":
			http.Redirect(w, r, "http://"+r.Host+"/index", http.StatusFound)
		default:
			_, _ = io.WriteString(w, sampleCustomIndex)
		}
	}))
	t.Cleanup(server.Close)
	client := registry.NewCustomIndexClient(server.Client())
	src := config.PluginRegistrySource{Name: "example", URL: server.URL + "/same-origin"}
	entries, err := client.Fetch(t.Context(), src)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	for _, path := range []string{"/external", "/insecure"} {
		src.URL = server.URL + path
		_, fetchErr := client.Fetch(t.Context(), src)
		require.ErrorContains(t, fetchErr, "redirect")
		assert.NotContains(t, fetchErr.Error(), server.URL)
	}
	assert.Zero(t, externalHits.Load())

	src.URL = "http://" + strings.TrimPrefix(server.URL, "https://")
	_, err = client.Fetch(t.Context(), src)
	require.ErrorContains(t, err, "HTTPS")
}

func TestCustomIndexFetchHonorsDeadline(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	src := config.PluginRegistrySource{Name: "example", URL: server.URL}
	_, err := registry.NewCustomIndexClient(server.Client()).Fetch(ctx, src)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
