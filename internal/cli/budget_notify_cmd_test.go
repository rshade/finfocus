package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

const notifyHint = "budget notifications for 1 exceeded threshold(s) were not sent; " +
	"pass --notify or set FINFOCUS_NOTIFY=true"

type notifyRequest struct {
	Path   string
	Header http.Header
	Body   map[string]any
}

type notifyMock struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests []notifyRequest
}

// newNotifyMock starts a TLS receiver that answers status and routes the
// finfocus notification client to it for the rest of the test.
func newNotifyMock(t *testing.T, status int) *notifyMock {
	t.Helper()
	mock := &notifyMock{}
	mock.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mock.mu.Lock()
		mock.requests = append(mock.requests, notifyRequest{Path: r.URL.Path, Header: r.Header.Clone(), Body: body})
		mock.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte("mock-response-body"))
	}))
	t.Cleanup(mock.srv.Close)
	t.Cleanup(cli.SetNotificationClientForTest(mock.srv.Client()))
	return mock
}

func (m *notifyMock) received() []notifyRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifyRequest(nil), m.requests...)
}

// notifyHome isolates FINFOCUS_HOME and the global config singleton, and
// writes cfg as the global config.hujson.
func notifyHome(t *testing.T, cfg string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_NOTIFY", "")
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"), []byte(cfg), 0o600))
	return home
}

// budgetWithAlert returns a $100 USD global budget with one actual alert. The
// sample plan prices at $0 without plugins, so threshold 0 is crossed and
// threshold 50 is not.
func budgetWithAlert(threshold float64, notifications string) string {
	return fmt.Sprintf(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":%g,"type":"actual","notifications":[%s]}]}}}}`, threshold, notifications)
}

func slackDest(url string) string {
	return fmt.Sprintf(`{"type":"slack","url":%q}`, url)
}

func runCLI(t *testing.T, args ...string) axtest.Result {
	t.Helper()
	config.ResetGlobalConfigForTest()
	return axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
}

func projectedArgs(extra ...string) []string {
	return append([]string{"cost", "projected", "--pulumi-json", simplePlanPath}, extra...)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyCostProjected(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL+"/services/T/B/X")))

	result := runCLI(t, projectedArgs("--notify")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	reqs := mock.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/services/T/B/X", reqs[0].Path)
	assert.Equal(t, "Budget alert: global budget crossed its 0% actual threshold", reqs[0].Body["text"])
	assert.NotContains(t, string(result.Stderr), mock.srv.URL)
	assert.NotContains(t, string(result.Stdout), mock.srv.URL)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyUnderThresholdSendsNothing(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(50, slackDest(mock.srv.URL)))

	result := runCLI(t, projectedArgs("--notify")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Empty(t, mock.received())

	result = runCLI(t, projectedArgs()...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.NotContains(t, string(result.Stderr), "budget notifications")
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyWithoutOptInPrintsHint(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL)))

	result := runCLI(t, projectedArgs()...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Empty(t, mock.received())
	assert.Equal(t, 1, strings.Count(string(result.Stderr), notifyHint), "stderr: %s", result.Stderr)
	assert.NotContains(t, string(result.Stdout), "budget notifications")
}

func TestBudgetNotifyEnvOptIn(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL)))
	t.Setenv("FINFOCUS_NOTIFY", "true")

	result := runCLI(t, projectedArgs()...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, mock.received(), 1)

	result = runCLI(t, projectedArgs("--notify=false")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, mock.received(), 1, "--notify=false overrides FINFOCUS_NOTIFY")
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyJSONOutputStaysClean(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL+"/services/SECRET")))

	result := runCLI(t, projectedArgs("--notify", "--output", "json")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.True(t, json.Valid(result.Stdout), "stdout: %s", result.Stdout)
	assert.NotContains(t, string(result.Stdout), "notifications")
	assert.NotContains(t, string(result.Stdout), "SECRET")
	assert.Len(t, mock.received(), 1)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyEveryRunSends(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL)))

	for range 2 {
		result := runCLI(t, projectedArgs("--notify")...)
		require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	}
	assert.Len(t, mock.received(), 2)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyTwoThresholds(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, fmt.Sprintf(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":0,"type":"actual","notifications":[%[1]s]},
  {"threshold":0.5,"type":"actual","notifications":[%[1]s]},
  {"threshold":0,"type":"actual","notifications":[%[1]s]}]}}}}`, slackDest(mock.srv.URL)))

	result := runCLI(t, projectedArgs("--notify")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, mock.received(), 2, "the 0.5%% threshold is not crossed at $0")
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyOverview(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL)))

	fixtures := filepath.Join("..", "..", "testdata", "overview")
	result := runCLI(t, "overview",
		"--pulumi-state", filepath.Join(fixtures, "state-mixed-changes.json"),
		"--pulumi-json", filepath.Join(fixtures, "plan-mixed-changes.json"),
		"--plain", "--yes", "--notify")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, mock.received(), 1)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyProjectConfigCannotExfiltrate(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, `{}`)
	secrets := map[string]string{
		"AWS_SECRET_ACCESS_KEY":     "aws-secret-value-123",
		"GITHUB_TOKEN":              "ghp_token_value_456",
		"FINFOCUS_NOTIFY_SLACK_URL": "https://hooks.slack.com/services/real-team-secret",
	}
	for name, value := range secrets {
		t.Setenv(name, value)
	}

	project := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".finfocus"), 0o700))
	attack := mock.srv.URL + "/collect"
	projectCfg := fmt.Sprintf(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":0,"type":"actual","notifications":[
    {"type":"webhook","url":"%[1]s?k=${AWS_SECRET_ACCESS_KEY}","headers":{"Authorization":"${GITHUB_TOKEN}"}},
    {"type":"webhook","url":"%[1]s","headers":{"X-Leak":"${FINFOCUS_NOTIFY_SLACK_URL}"}},
    {"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}"}]}]}}}}`, attack)
	require.NoError(t, os.WriteFile(filepath.Join(project, ".finfocus", "config.hujson"), []byte(projectCfg), 0o600))

	result := runCLI(t, projectedArgs("--notify", "--project-dir", project)...)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Empty(t, mock.received())
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "project config")
	for _, value := range secrets {
		assert.NotContains(t, stderr, value)
		assert.NotContains(t, string(result.Stdout), value)
	}
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyFailureKeepsOutputAndExitCode(t *testing.T) {
	mock := newNotifyMock(t, http.StatusInternalServerError)
	secretURL := mock.srv.URL + "/services/T/B/failure-secret"
	webhook := fmt.Sprintf(`{"type":"webhook","url":%q,"headers":{"Authorization":"Bearer hdr-secret"}}`, mock.srv.URL)

	cases := []struct {
		name string
		args []string
	}{
		{name: "table", args: projectedArgs()},
		{name: "exit on threshold", args: projectedArgs("--exit-on-threshold", "--exit-code", "3")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notifyHome(t, budgetWithAlert(0, ""))
			baseline := runCLI(t, tc.args...)

			notifyHome(t, budgetWithAlert(0, slackDest(secretURL)+","+webhook))
			got := runCLI(t, append(append([]string{}, tc.args...), "--notify")...)

			assert.Equal(t, baseline.ExitCode, got.ExitCode)
			assert.Equal(t, string(baseline.Stdout), string(got.Stdout))
			stderr := string(got.Stderr)
			assert.Contains(
				t,
				stderr,
				"warning: slack notification for global budget (0% actual) failed: unexpected response status: status 500",
			)
			assert.Contains(
				t,
				stderr,
				"warning: webhook notification for global budget (0% actual) failed: unexpected response status: status 500",
			)
			for _, secret := range []string{mock.srv.URL, "failure-secret", "hdr-secret", "mock-response-body"} {
				assert.NotContains(t, stderr, secret)
			}
		})
	}
	assert.Len(t, mock.received(), 4)
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyExitCodeWithExitOnThreshold(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL)))

	result := runCLI(t, projectedArgs("--notify", "--exit-on-threshold", "--exit-code", "3")...)
	assert.Equal(t, 3, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Len(t, mock.received(), 1)
}

func TestBudgetNotifyUnsetVariableSkips(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest("${FINFOCUS_NOTIFY_MISSING_URL}")+","+slackDest(mock.srv.URL)))
	t.Setenv("FINFOCUS_NOTIFY_MISSING_URL", "")

	result := runCLI(t, projectedArgs("--notify")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Contains(t, string(result.Stderr),
		"warning: slack notification for global budget (0% actual) skipped: url: variable is unset or empty: "+
			"FINFOCUS_NOTIFY_MISSING_URL")
	assert.Len(t, mock.received(), 1, "the other destination still sends")
}

//nolint:paralleltest // sets env, the global config singleton, and the notification client seam
func TestBudgetNotifyDryRun(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	notifyHome(t, budgetWithAlert(0, slackDest(mock.srv.URL+"/dry-secret")+","+
		fmt.Sprintf(`{"type":"webhook","url":%q}`, mock.srv.URL)))

	result := runCLI(t, projectedArgs("--notify", "--dry-run")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	assert.Empty(t, mock.received())
	stderr := string(result.Stderr)
	assert.Equal(t, 1, strings.Count(stderr,
		"dry-run: would notify slack for global budget, 0% actual threshold"), stderr)
	assert.Equal(t, 1, strings.Count(stderr,
		"dry-run: would notify webhook for global budget, 0% actual threshold"), stderr)
	assert.NotContains(t, stderr, mock.srv.URL)
	assert.NotContains(t, string(result.Stdout), "dry-run: would notify")
}

func TestBudgetNotifyWebhookHeaderFromEnv(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	token := "api-token-value-789"
	t.Setenv("FINFOCUS_NOTIFY_API_TOKEN", token)
	notifyHome(t, budgetWithAlert(0, fmt.Sprintf(
		`{"type":"webhook","url":%q,"headers":{"Authorization":"Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"}}`,
		mock.srv.URL+"/budget-alert")))

	result := runCLI(t, projectedArgs("--notify", "--debug")...)
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	reqs := mock.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, "Bearer "+token, reqs[0].Header.Get("Authorization"))
	assert.Equal(t, "budget.threshold.exceeded", reqs[0].Body["event"])
	assert.NotContains(t, string(result.Stderr), token)
	assert.NotContains(t, string(result.Stdout), token)
}

func TestBudgetNotifyWebhookRejectsOtherVariable(t *testing.T) {
	mock := newNotifyMock(t, http.StatusOK)
	t.Setenv("API_TOKEN", "should-never-be-read")
	notifyHome(t, budgetWithAlert(0, fmt.Sprintf(
		`{"type":"webhook","url":%q,"headers":{"Authorization":"Bearer ${API_TOKEN}"}}`, mock.srv.URL)))

	result := runCLI(t, projectedArgs("--notify")...)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "API_TOKEN")
	assert.NotContains(t, string(result.Stderr), "should-never-be-read")
	assert.Empty(t, mock.received())
}
