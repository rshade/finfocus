package cli

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cloudhistory"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		mode    string
		message string
	}{
		{"unset", []string{"--import-cloud-history"}, "", "Pulumi Cloud history is disabled; set cost.history.pulumi_cloud.enabled to auto or true"},
		{"off", []string{"--import-cloud-history"}, "false", "Pulumi Cloud history is disabled"},
		{"from alone", []string{"--from", "2026-01-01"}, "auto", "--import-cloud-history"},
		{"to alone", []string{"--to", "2026-01-01"}, "auto", "--import-cloud-history"},
		{"web", []string{"--import-cloud-history", "--web"}, "auto", "--web"},
		{"mcp", []string{"--import-cloud-history", "--mcp"}, "auto", "--mcp"},
		{"subcommand", []string{"--import-cloud-history", "config", "list"}, "auto", "import-cloud-history"},
		{"bad date", []string{"--import-cloud-history", "--from", "invalid"}, "auto", "date"},
		{"bad format", []string{"--import-cloud-history", "--format", "xml"}, "auto", "mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupCloudModeProject(t, "auto")
			if tc.mode == "" {
				require.NoError(t, os.WriteFile(".finfocus/config.hujson", []byte(`{}`), 0600))
			} else {
				contents := fmt.Sprintf(`{"cost":{"history":{"pulumi_cloud":{"enabled":"%s"}}}}`, tc.mode)
				require.NoError(t, os.WriteFile(".finfocus/config.hujson", []byte(contents), 0600))
			}
			var requests atomic.Int64
			server := httptest.NewServer(
				http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }),
			)
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			result := axtest.Run(context.Background(), t, NewRootCmd("test"), tc.args)
			assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
			assert.Contains(t, string(result.Stderr), tc.message)
			assert.Zero(t, requests.Load())
		})
	}
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryOutput(t *testing.T) {
	setupCloudModeProject(t, "auto")
	t.Setenv("FINFOCUS_HISTORY_RETENTION_DAYS", "10")
	at := time.Now().Add(-time.Hour).Unix()
	var exports atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/user":
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/updates"):
			if r.URL.Query().Get("page") == "1" {
				_, _ = fmt.Fprintf(
					w,
					`{"updates":[{"version":1,"kind":"update","endTime":%d}]}`,
					at,
				)
			} else {
				_, _ = w.Write([]byte(`{"updates":[]}`))
			}
		case strings.Contains(r.URL.Path, "/export/"):
			exports.Add(1)
			_, _ = w.Write(
				[]byte(
					`{"deployment":{"resources":[{"urn":"urn:pulumi:dev::test::aws:ec2/instance:Instance::old","id":"i-old","type":"aws:ec2/instance:Instance","custom":true}]}}`,
				),
			)
		default:
			assert.Fail(t, "unexpected request", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	args := []string{
		"--import-cloud-history",
		"--stack",
		"dev",
		"--from",
		time.Now().Add(-20 * 24 * time.Hour).Format(time.RFC3339),
	}
	first := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
	require.Equal(t, 0, first.ExitCode, string(first.Stderr))
	assert.Equal(
		t,
		"Updates listed: 1\nSkipped (not state changing): 0\nServed from cache: 0\nFetched: 1\nIdentities recorded: 1\nUnreadable: 0\n",
		string(first.Stdout),
	)
	assert.Contains(t, string(first.Stderr), "retention")
	second := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		append(args, "--format", "json"),
	)
	require.Equal(t, 0, second.ExitCode, string(second.Stderr))
	var got cloudhistory.Result
	require.NoError(t, json.Unmarshal(second.Stdout, &got))
	assert.Equal(t, 1, got.FromCache)
	assert.Zero(t, got.Fetched)
	assert.Equal(t, int64(1), exports.Load())
	assert.True(t, got.Clamped)
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryFailures(t *testing.T) {
	for _, failure := range []string{"disabled store", "locked store", "unreachable", "unreadable"} {
		t.Run(failure, func(t *testing.T) {
			setupCloudModeProject(t, "auto")
			want := 1
			message := "unreachable"
			switch failure {
			case "disabled store":
				t.Setenv("FINFOCUS_HISTORY_ENABLED", "false")
				want = 2
				message = "cost.history.enabled"
			case "locked store":
				dir := filepath.Join(config.ResolveConfigDir(), "history")
				store, err := history.NewBoltStore(context.Background(), dir, true, 90)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				message = "lock"
			}
			server := httptest.NewServer(http.HandlerFunc(serveImportHistoryFailure))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			if failure == "unreachable" {
				server.Close()
			}
			if failure == "unreadable" {
				message = "unreadable"
			}
			result := axtest.Run(
				context.Background(),
				t,
				NewRootCmd("test"),
				[]string{"--import-cloud-history"},
			)
			assert.Equal(t, want, result.ExitCode, string(result.Stderr))
			assert.Contains(t, string(result.Stderr), message)
			assert.NotContains(t, string(result.Stdout), "Total")
			if failure == "disabled store" {
				assert.Contains(t, string(result.Stderr), "cost.history.pulumi_cloud.enabled")
			}
		})
	}
}

// Root command construction and configuration use process-wide state.
func TestImportCloudHistorySchema(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"__schema"})
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	var schema struct {
		Command map[string]json.RawMessage `json:"command"`
	}
	require.NoError(t, json.Unmarshal(result.Stdout, &schema))
	// Pin the root surface; each subcommand keeps its existing tests and MCP policy.
	delete(schema.Command, "commands")
	got, err := json.MarshalIndent(schema.Command, "", "  ")
	require.NoError(t, err)
	got = append(got, '\n')
	path := filepath.Join("testdata", "schema-root.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(path, got, 0600))
	}
	want, err := os.ReadFile(path)
	require.NoError(
		t,
		err,
		"run UPDATE_GOLDEN=1 go test ./internal/cli -run TestImportCloudHistorySchema",
	)
	assert.JSONEq(t, string(want), string(got))
}

func serveImportHistoryFailure(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/user" {
		_, _ = w.Write([]byte(`{}`))
		return
	}
	if strings.HasSuffix(r.URL.Path, "/updates") {
		if r.URL.Query().Get("page") == "1" {
			_, _ = fmt.Fprintf(
				w,
				`{"updates":[{"version":1,"kind":"update","endTime":%d}]}`,
				time.Now().Add(-time.Hour).Unix(),
			)
		} else {
			_, _ = w.Write([]byte(`{"updates":[]}`))
		}
		return
	}
	http.NotFound(w, r)
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryMachineOutputModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		env  string
	}{
		{"AGENT_MODE json", nil, "json"},
		{"AGENT_MODE one", nil, "1"},
		{"agent alias", []string{"--format", "agent"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupCloudModeProject(t, "auto")
			t.Setenv("AGENT_MODE", tc.env)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/user" {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_, _ = w.Write([]byte(`{"updates":[]}`))
			}))
			t.Cleanup(server.Close)
			t.Setenv("PULUMI_BACKEND_URL", server.URL)
			args := append([]string{"--import-cloud-history", "--stack", "dev"}, tc.args...)
			result := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))
			var got cloudhistory.Result
			require.NoError(t, json.Unmarshal(result.Stdout, &got), string(result.Stdout))
		})
	}
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryDryRunWritesNothing(t *testing.T) {
	setupCloudModeProject(t, "auto")
	var requests atomic.Int64
	server := httptest.NewServer(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }),
	)
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)
	historyDir := filepath.Join(config.ResolveConfigDir(), "history")

	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"--import-cloud-history", "--dry-run"},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Would import Pulumi Cloud history")
	assert.Zero(t, requests.Load())
	assert.NoDirExists(t, historyDir)

	machine := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"--import-cloud-history", "--dry-run", "--format", "json"},
	)
	require.Equal(t, 0, machine.ExitCode, string(machine.Stderr))
	assert.Contains(t, string(machine.Stdout), `"would_import_cloud_history"`)
	assert.NoDirExists(t, historyDir)
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryNonCloudBackendIsValidationError(t *testing.T) {
	setupCloudModeProject(t, "auto")
	t.Setenv("PULUMI_BACKEND_URL", "file://"+t.TempDir())
	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"--import-cloud-history", "--stack", "dev"},
	)
	assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stderr), "non_cloud_backend")
}

// Root commands and project configuration use process-wide state.
func TestImportCloudHistoryProjectDirDoesNotChangeDetectedProject(t *testing.T) {
	setupCloudModeProject(t, "auto")
	other := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(other, "Pulumi.yaml"), []byte("name: other\nruntime: yaml\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(other, ".finfocus"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(other, ".finfocus", "config.hujson"),
		[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"auto"}}}}`),
		0o600,
	))
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/updates") {
			_, _ = w.Write([]byte(`{"updates":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("PULUMI_BACKEND_URL", server.URL)

	result := axtest.Run(
		context.Background(),
		t,
		NewRootCmd("test"),
		[]string{"--import-cloud-history", "--stack", "dev", "--project-dir", other},
	)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, paths, "/api/stacks/org/test/dev/updates")
	for _, path := range paths {
		assert.NotContains(t, path, "/other/", "the import must use the working-directory project")
	}
}
