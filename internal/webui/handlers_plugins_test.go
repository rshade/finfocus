package webui

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestPluginCatalogAndOrigin(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{
		PluginCatalog: func(context.Context) ([]PluginCatalogRow, error) {
			return []PluginCatalogRow{{
				Name: "aws-public", Providers: []string{"aws"}, Capabilities: []string{"projected"},
				InstalledVersions: []string{}, InCatalog: true, SecurityLevel: "community",
			}}, nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	response := overviewRequest(t, server, http.MethodGet, "/api/plugins/catalog", "")
	body := readBody(t, response)
	assert.Contains(t, body, `"name":"aws-public"`)
	assert.Contains(t, body, `"providers":["aws"]`)

	req, err := http.NewRequest(
		http.MethodPost,
		server.BaseURL()+"/api/plugins/aws-public/install",
		strings.NewReader(`{}`),
	)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: server.cookieName(), Value: server.token})
	req.Header.Set("Content-Type", "application/json")
	denied, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, denied.Body.Close()) })
	assert.Equal(t, http.StatusForbidden, denied.StatusCode)
}

func TestPluginPlanRejectsBadInput(t *testing.T) {
	t.Parallel()
	called := false
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(context.Context, string, PluginPlanRequest) (PluginPlan, error) {
			called = true
			return PluginPlan{}, nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	unknown := pluginRequest(t, server, http.MethodPost, "/api/plugins/aws-public/plan", `{"action":"inspect"}`)
	assert.Equal(t, http.StatusBadRequest, unknown.StatusCode)
	missing := pluginRequest(t, server, http.MethodPost, "/api/plugins/aws-public/plan", `{"action":"remove"}`)
	assert.Equal(t, http.StatusBadRequest, missing.StatusCode)
	slash := pluginRequest(t, server, http.MethodPost, "/api/plugins/AWS/plan", `{"action":"install"}`)
	assert.Equal(t, http.StatusBadRequest, slash.StatusCode)
	assert.False(t, called)
}

func TestPluginInstallConfirmation(t *testing.T) {
	t.Parallel()
	plan := PluginPlan{
		Action: "install", Name: "recorder", Version: "v1.0.0",
		Repository: "rshade/finfocus", SecurityLevel: "official",
		ChecksumFile: "present", InCatalog: true,
	}
	var mu sync.Mutex
	var installs int
	var saw PluginOperationRequest
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(_ context.Context, _ string, request PluginPlanRequest) (PluginPlan, error) {
			if request.Version == "v2.0.0" {
				return PluginPlan{
					Action: request.Action, Name: "recorder", Version: "v1.0.0",
					Repository: plan.Repository, SecurityLevel: plan.SecurityLevel,
					FallbackFrom: "v2.0.0", ChecksumFile: "present", InCatalog: true,
				}, nil
			}
			return plan, nil
		},
		PluginInstall: func(_ context.Context, _ string, request PluginOperationRequest, _ func(PluginProgress)) error {
			mu.Lock()
			installs++
			saw = request
			mu.Unlock()
			return nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	body := `{"version":"v1.0.0","repository":"rshade/finfocus","securityLevel":"official","acceptMissingChecksum":false,"acceptFallbackFrom":""}`
	okResp := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install", body)
	assert.Equal(t, http.StatusAccepted, okResp.StatusCode)
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return installs == 1 && saw.CatalogOnly && saw.RequireChecksumFile
	}, time.Second, 10*time.Millisecond)

	mismatch := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install",
		`{"version":"v9.0.0","repository":"rshade/finfocus","securityLevel":"official"}`)
	assert.Equal(t, http.StatusConflict, mismatch.StatusCode)

	fallback := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install",
		`{"version":"v2.0.0","repository":"rshade/finfocus","securityLevel":"official","acceptFallbackFrom":""}`)
	assert.Equal(t, http.StatusConflict, fallback.StatusCode)
	mu.Lock()
	assert.Equal(t, 1, installs)
	mu.Unlock()
}

func TestPluginInstallMissingChecksumAndBusy(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	var installs int
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(_ context.Context, name string, request PluginPlanRequest) (PluginPlan, error) {
			checksum := "present"
			if name == "missing-sum" {
				checksum = "missing"
			}
			return PluginPlan{
				Action: request.Action, Name: name, Version: "v1.0.0",
				Repository: "rshade/finfocus", SecurityLevel: "official",
				ChecksumFile: checksum, InCatalog: true,
			}, nil
		},
		PluginInstall: func(context.Context, string, PluginOperationRequest, func(PluginProgress)) error {
			installs++
			if installs == 1 {
				<-hold
			}
			return nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	missing := pluginRequest(t, server, http.MethodPost, "/api/plugins/missing-sum/install",
		`{"version":"v1.0.0","repository":"rshade/finfocus","securityLevel":"official","acceptMissingChecksum":false}`)
	assert.Equal(t, http.StatusConflict, missing.StatusCode)
	assert.Equal(t, 0, installs)

	first := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install",
		`{"version":"v1.0.0","repository":"rshade/finfocus","securityLevel":"official"}`)
	require.Equal(t, http.StatusAccepted, first.StatusCode)
	second := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install",
		`{"version":"v1.0.0","repository":"rshade/finfocus","securityLevel":"official"}`)
	assert.Equal(t, http.StatusConflict, second.StatusCode)
	close(hold)
}

func TestPluginOperationStream(t *testing.T) {
	t.Parallel()
	snapshotReady := make(chan struct{})
	release := make(chan struct{})
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(_ context.Context, _ string, request PluginPlanRequest) (PluginPlan, error) {
			return PluginPlan{
				Action: request.Action, Name: "recorder", Version: "v1.0.0",
				Repository: "rshade/finfocus", SecurityLevel: "official",
				ChecksumFile: "present", InCatalog: true,
			}, nil
		},
		PluginInstall: func(_ context.Context, _ string, _ PluginOperationRequest, report func(PluginProgress)) error {
			<-snapshotReady
			report(PluginProgress{Phase: "download", Message: "Downloading", Bytes: 3, Total: 9})
			<-release
			return assert.AnError
		},
	})
	server := newRunningServer(t, Options{Session: session})
	accepted := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/install",
		`{"version":"v1.0.0","repository":"rshade/finfocus","securityLevel":"official"}`)
	require.Equal(t, http.StatusAccepted, accepted.StatusCode)
	var body PluginAccepted
	require.NoError(t, json.NewDecoder(accepted.Body).Decode(&body))
	require.NotEmpty(t, body.OperationID)

	streamReq, err := http.NewRequest(
		http.MethodGet,
		server.BaseURL()+"/api/plugins/operations/"+body.OperationID+"/stream",
		nil,
	)
	require.NoError(t, err)
	streamReq.AddCookie(&http.Cookie{Name: server.cookieName(), Value: server.token})
	stream, err := http.DefaultClient.Do(streamReq)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, stream.Body.Close()) })
	reader := bufio.NewReader(stream.Body)
	first := readOneSSE(t, reader)
	assert.Equal(t, "snapshot", first.name)
	close(snapshotReady)
	second := readOneSSE(t, reader)
	assert.Equal(t, "progress", second.name)
	assert.Contains(t, second.data, `"bytes":3`)
	close(release)
	third := readOneSSE(t, reader)
	assert.Equal(t, "done", third.name)
	assert.Contains(t, third.data, `"failed"`)
	assert.NotContains(t, third.data, "http://")

	missing := pluginRequest(t, server, http.MethodGet, "/api/plugins/operations/missing/stream", "")
	assert.Equal(t, http.StatusNotFound, missing.StatusCode)
	assert.Contains(t, readBody(t, missing), "operation not found")
}

func TestPluginSettingsHideSecretsAndRejectForeignKeys(t *testing.T) {
	t.Parallel()
	prepared := 0
	session := NewSession(context.Background(), SessionOptions{
		PluginSettings: func(context.Context, string) (PluginSettingsView, error) {
			return PluginSettingsView{
				Name: "jev", Mode: "descriptor",
				Descriptors: []PluginSettingDescriptor{{
					Key: "api_key", Type: "string", Secret: true, Default: "super-secret", DefaultSet: true,
				}},
				Values: []PluginSettingValue{{
					Key: "api_key", Secret: true,
					Project: PluginValueSide{Set: true, Value: "super-secret"},
					Global:  PluginValueSide{Set: true, Value: "super-secret"},
				}},
				Versions: map[string]string{"global": "abc", "project": "absent"},
			}, nil
		},
		PluginSettingsPrepare: func(
			_ context.Context, _ string, request config.WriteRequest,
		) (config.WriteRequest, error) {
			prepared++
			filtered := request
			filtered.Changes = nil
			return filtered, nil
		},
		ConfigSave: func(context.Context, config.WriteRequest) (config.PatchResult, error) {
			return config.PatchResult{Version: "next"}, nil
		},
		ConfigPreview: func(context.Context, config.WriteRequest) (config.PatchResult, error) {
			return config.PatchResult{}, nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	got := pluginRequest(t, server, http.MethodGet, "/api/plugins/jev/settings", "")
	body := readBody(t, got)
	assert.NotContains(t, body, "super-secret")
	assert.Contains(t, body, `"defaultSet":true`)

	foreign := pluginRequest(t, server, http.MethodPost, "/api/plugins/jev/settings",
		`{"tier":"global","baseVersion":"abc","changes":[{"key":"plugins.other.api_key","value":"x"}]}`)
	assert.Equal(t, http.StatusBadRequest, foreign.StatusCode)
	assert.Equal(t, 0, prepared)

	saved := pluginRequest(t, server, http.MethodPost, "/api/plugins/jev/settings",
		`{"tier":"global","baseVersion":"abc","changes":[{"key":"plugins.jev.api_key","value":"next-secret"}]}`)
	assert.Equal(t, http.StatusOK, saved.StatusCode)
	savedBody := readBody(t, saved)
	assert.Contains(t, savedBody, pluginSettingsSaved)
	assert.NotContains(t, savedBody, "super-secret")
	assert.NotContains(t, savedBody, "next-secret")
	assert.Equal(t, 1, prepared)
}

func TestPluginSettingsIncludeESCReference(t *testing.T) {
	t.Parallel()
	view := PluginSettingsView{
		Name: "recorder", Mode: "descriptor",
		Descriptors: []PluginSettingDescriptor{{
			Key: "output_dir", Type: "string", Env: "FINFOCUS_RECORDER_OUTPUT_DIR",
		}},
		Values: []PluginSettingValue{
			{
				Key: "output_dir", EffectiveSource: "esc", ESCEnvironment: "billing/recorder",
				EnvSet: true,
			},
			{
				Key: "note", Secret: true, EffectiveSource: "esc", ESCEnvironment: "billing/recorder",
				Project: PluginValueSide{Set: true, Value: "super-secret"},
			},
		},
		Versions: map[string]string{"global": "absent", "project": "absent"},
	}
	session := NewSession(context.Background(), SessionOptions{
		PluginSettings: func(context.Context, string) (PluginSettingsView, error) {
			return view, nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	got := pluginRequest(t, server, http.MethodGet, "/api/plugins/recorder/settings", "")
	body := readBody(t, got)
	assert.Contains(t, body, `"escEnvironment":"billing/recorder"`)
	assert.NotContains(t, body, "super-secret")
	assert.NotContains(t, body, `"value"`)
	plain, err := json.Marshal(stripSecretSettings(view))
	require.NoError(t, err)
	assert.JSONEq(t, string(plain), body)
	redacted, err := RedactJSON(stripSecretSettings(view))
	require.NoError(t, err)
	assert.Contains(t, string(redacted), `"escEnvironment":"billing/recorder"`)
	assert.NotContains(t, string(redacted), "super-secret")
}

func TestPluginValidateNotInstalled(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{
		PluginValidate: func(context.Context, string) (PluginValidateResult, error) {
			return PluginValidateResult{}, &StatusError{
				Status: http.StatusNotFound,
				Code:   "not_found",
				Text:   "plugin is not installed",
			}
		},
	})
	server := newRunningServer(t, Options{Session: session})
	response := pluginRequest(t, server, http.MethodPost, "/api/plugins/recorder/validate", `{}`)
	assert.Equal(t, http.StatusNotFound, response.StatusCode)
	passed := NewSession(context.Background(), SessionOptions{
		PluginValidate: func(context.Context, string) (PluginValidateResult, error) {
			return PluginValidateResult{Passed: false, Reason: "plugin binary is not executable"}, nil
		},
	})
	running := newRunningServer(t, Options{Session: passed})
	fail := pluginRequest(t, running, http.MethodPost, "/api/plugins/recorder/validate", `{}`)
	assert.Equal(t, http.StatusOK, fail.StatusCode)
	assert.Contains(t, readBody(t, fail), `"passed":false`)
}

func TestPluginNonCatalogPlan(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{
		PluginPlan: func(context.Context, string, PluginPlanRequest) (PluginPlan, error) {
			return PluginPlan{}, &StatusError{
				Status: http.StatusNotFound,
				Code:   "not_found",
				Text:   "plugin is not in the catalog",
			}
		},
		PluginInstall: func(context.Context, string, PluginOperationRequest, func(PluginProgress)) error {
			assert.Fail(t, "install must not run")
			return nil
		},
	})
	server := newRunningServer(t, Options{Session: session})
	response := pluginRequest(t, server, http.MethodPost, "/api/plugins/custom/install",
		`{"version":"v1.0.0","repository":"","securityLevel":""}`)
	assert.Equal(t, http.StatusNotFound, response.StatusCode)
}

func pluginRequest(t *testing.T, server *Server, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, server.BaseURL()+path, strings.NewReader(body))
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: server.cookieName(), Value: server.token})
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", server.BaseURL())
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, resp.Body.Close()) })
	return resp
}

type sseEvent struct {
	name string
	data string
}

func readOneSSE(t *testing.T, reader *bufio.Reader) sseEvent {
	t.Helper()
	type result struct {
		event sseEvent
		err   error
	}
	done := make(chan result, 1)
	go func() {
		var event sseEvent
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- result{err: err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				event.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				event.data = strings.TrimPrefix(line, "data: ")
			case line == "" && event.name != "":
				done <- result{event: event}
				return
			}
		}
	}()
	select {
	case got := <-done:
		require.NoError(t, got.err)
		return got.event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an SSE event")
		return sseEvent{}
	}
}
