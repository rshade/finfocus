package integration_test

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notificationReceiver struct {
	srv      *httptest.Server
	certFile string
	status   int

	mu       sync.Mutex
	requests []receivedNotification
}

type receivedNotification struct {
	Path   string
	Header http.Header
	Body   map[string]any
}

// newNotificationReceiver starts a TLS mock and writes its certificate to a
// PEM file that the finfocus binary trusts through SSL_CERT_FILE.
func newNotificationReceiver(t *testing.T) *notificationReceiver {
	t.Helper()
	recv := &notificationReceiver{status: http.StatusOK}
	recv.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		recv.mu.Lock()
		recv.requests = append(
			recv.requests,
			receivedNotification{Path: r.URL.Path, Header: r.Header.Clone(), Body: body},
		)
		status := recv.status
		recv.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(recv.srv.Close)

	recv.certFile = filepath.Join(t.TempDir(), "mock-ca.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: recv.srv.Certificate().Raw})
	require.NoError(t, os.WriteFile(recv.certFile, certPEM, 0o600))
	return recv
}

func (r *notificationReceiver) setStatus(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

func (r *notificationReceiver) take() []receivedNotification {
	r.mu.Lock()
	defer r.mu.Unlock()
	got := r.requests
	r.requests = nil
	return got
}

type notifyRun struct {
	stdout   string
	stderr   string
	exitCode int
}

//nolint:paralleltest // builds a binary and runs it with a per-test FINFOCUS_HOME
func TestBudgetNotifications(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("SSL_CERT_FILE trust for the TLS mock is only honored on Linux")
	}
	if testing.Short() {
		t.Skip("skipping binary-building notification test in short mode")
	}

	binary := filepath.Join(t.TempDir(), "finfocus")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/finfocus")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)

	plan, err := filepath.Abs("../../examples/plans/aws-simple-plan.json")
	require.NoError(t, err)
	recv := newNotificationReceiver(t)

	run := func(t *testing.T, globalCfg string, extraEnv []string, args ...string) notifyRun {
		t.Helper()
		home := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(globalCfg), 0o600))
		cmd := exec.Command(binary, append([]string{"cost", "projected", "--pulumi-json", plan}, args...)...)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(),
			"FINFOCUS_HOME="+home,
			"PULUMI_HOME="+filepath.Join(home, "pulumi"),
			"FINFOCUS_SKIP_MIGRATION_CHECK=1",
			"FINFOCUS_PROJECT_DIR=",
			"FINFOCUS_NOTIFY=",
			"SSL_CERT_FILE="+recv.certFile,
		)
		cmd.Env = append(cmd.Env, extraEnv...)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		code := 0
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			require.NoError(t, runErr)
		}
		return notifyRun{stdout: stdout.String(), stderr: stderr.String(), exitCode: code}
	}

	token := "integration-token-value"
	globalCfg := fmt.Sprintf(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":0,"type":"actual","notifications":[
    {"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}"},
    {"type":"webhook","url":%q,"headers":{"Authorization":"Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"}}
  ]}]}}}}`, recv.srv.URL+"/budget-alert")
	notifyEnv := []string{
		"FINFOCUS_NOTIFY_SLACK_URL=" + recv.srv.URL + "/services/T000/B000/slack-secret",
		"FINFOCUS_NOTIFY_API_TOKEN=" + token,
	}

	t.Run("slack and webhook deliveries", func(t *testing.T) {
		recv.setStatus(http.StatusOK)
		result := run(t, globalCfg, notifyEnv, "--notify")
		require.Equal(t, 0, result.exitCode, "stderr: %s", result.stderr)

		got := recv.take()
		require.Len(t, got, 2)
		byPath := map[string]receivedNotification{}
		for _, req := range got {
			byPath[req.Path] = req
		}
		slack := byPath["/services/T000/B000/slack-secret"]
		assert.Equal(t, "Budget alert: global budget crossed its 0% actual threshold", slack.Body["text"])
		webhook := byPath["/budget-alert"]
		assert.Equal(t, "budget.threshold.exceeded", webhook.Body["event"])
		assert.Equal(t, "Bearer "+token, webhook.Header.Get("Authorization"))
		for _, secret := range []string{"slack-secret", token} {
			assert.NotContains(t, result.stderr, secret)
			assert.NotContains(t, result.stdout, secret)
		}
	})

	t.Run("no delivery without opt-in", func(t *testing.T) {
		recv.setStatus(http.StatusOK)
		result := run(t, globalCfg, notifyEnv)
		require.Equal(t, 0, result.exitCode, "stderr: %s", result.stderr)
		assert.Empty(t, recv.take())
		assert.Contains(t, result.stderr, "pass --notify or set FINFOCUS_NOTIFY=true")
	})

	t.Run("http 500 keeps the exit code", func(t *testing.T) {
		recv.setStatus(http.StatusInternalServerError)
		args := []string{"--exit-on-threshold", "--exit-code", "4"}
		baseline := run(t, `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD",
  "alerts":[{"threshold":0,"type":"actual"}]}}}}`, nil, args...)
		result := run(t, globalCfg, notifyEnv, append(args, "--notify")...)

		assert.Equal(t, 4, baseline.exitCode, "stderr: %s", baseline.stderr)
		assert.Equal(t, baseline.exitCode, result.exitCode)
		assert.Equal(t, baseline.stdout, result.stdout)
		assert.Contains(t, result.stderr, "warning: slack notification for global budget (0% actual) failed")
		assert.Contains(t, result.stderr, "status 500")
		assert.NotContains(t, result.stderr, "slack-secret")
		assert.NotContains(t, result.stderr, token)
		assert.Len(t, recv.take(), 2)
	})

	t.Run("project config referencing GITHUB_TOKEN is rejected", func(t *testing.T) {
		recv.setStatus(http.StatusOK)
		project := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(project, ".finfocus"), 0o700))
		projectCfg := fmt.Sprintf(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":0,"type":"actual","notifications":[
    {"type":"webhook","url":%q,"headers":{"Authorization":"${GITHUB_TOKEN}"}}]}]}}}}`, recv.srv.URL+"/collect")
		require.NoError(
			t,
			os.WriteFile(filepath.Join(project, ".finfocus", "config.hujson"), []byte(projectCfg), 0o600),
		)

		ghToken := "ghp_integration_secret"
		result := run(t, `{}`, []string{"GITHUB_TOKEN=" + ghToken}, "--notify", "--project-dir", project)
		assert.NotEqual(t, 0, result.exitCode)
		assert.Contains(t, result.stderr, "GITHUB_TOKEN")
		assert.NotContains(t, result.stderr, ghToken)
		assert.NotContains(t, result.stdout, ghToken)
		assert.Empty(t, recv.take())
	})
}
