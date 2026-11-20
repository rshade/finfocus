package integration_test

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

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

	t.Run("email obeys opt-in, dry run, and stdout contract", func(t *testing.T) {
		port, messages := newBudgetSMTPRelay(t, false)
		cfg := emailBudgetConfig(port)
		env := []string{
			"FINFOCUS_NOTIFY_SMTP_USER=relay-login-secret",
			"FINFOCUS_NOTIFY_SMTP_PASS=relay-password-secret",
		}

		baseline := run(t, cfg, env, "--output", "json")
		require.Zero(t, baseline.exitCode, baseline.stderr)
		assert.Contains(t, baseline.stderr, "pass --notify or set FINFOCUS_NOTIFY=true")
		assert.Empty(t, messages)

		preview := run(t, cfg, env, "--output", "json", "--notify", "--dry-run")
		require.Zero(t, preview.exitCode, preview.stderr)
		assert.Contains(t, preview.stderr, "dry-run: would notify email")
		assert.Contains(t, preview.stderr, "To: owner@example.com; Cc: finance@example.com")
		assert.Empty(t, messages)

		env = append(env, "FINFOCUS_NOTIFY=true")
		sent := run(t, cfg, env, "--output", "json", "--debug")
		require.Zero(t, sent.exitCode, sent.stderr)
		require.True(t, json.Valid([]byte(sent.stdout)))
		assert.NotContains(t, sent.stderr, "warning: email notification")
		select {
		case message := <-messages:
			assert.Contains(t, message, "Subject: [FinFocus] Budget Alert: 0% threshold exceeded")
			assert.Contains(t, message, "To: <owner@example.com>")
			assert.Contains(t, message, "Cc: <finance@example.com>")
		case <-time.After(time.Second):
			t.Fatal("email was not delivered")
		}
		assert.Empty(t, messages)
		for _, result := range []notifyRun{baseline, preview, sent} {
			assert.NotContains(t, result.stdout+result.stderr, "relay-login-secret")
			assert.NotContains(t, result.stdout+result.stderr, "relay-password-secret")
		}
	})

	t.Run("email failure keeps budget exit status", func(t *testing.T) {
		port, messages := newBudgetSMTPRelay(t, true)
		result := run(t, emailBudgetConfig(port), []string{
			"FINFOCUS_NOTIFY_SMTP_USER=relay-login-secret", "FINFOCUS_NOTIFY_SMTP_PASS=relay-password-secret",
		}, "--output", "json", "--notify", "--exit-on-threshold", "--exit-code", "4")
		assert.Equal(t, 4, result.exitCode, result.stderr)
		assert.True(t, json.Valid([]byte(result.stdout)))
		assert.Equal(t, 1, strings.Count(result.stderr, "warning: email notification"))
		assert.NotContains(t, result.stderr, "budget notification not delivered")
		assert.Contains(t, result.stderr, "SMTP recipient rejected (SMTP 550)")
		assert.NotContains(t, result.stdout+result.stderr, "relay-login-secret")
		assert.NotContains(t, result.stdout+result.stderr, "relay-password-secret")
		assert.Empty(t, messages)
	})

	t.Run("pagerduty opt-in and deduplication", func(t *testing.T) {
		recv.setStatus(http.StatusAccepted)
		const routingKey = "test-pagerduty-integration-key"
		cfg := fmt.Sprintf(`{"logging":{"file":""},"cost":{"budgets":{"global":{"amount":100,"currency":"USD",
		  "alerts":[{"type":"actual","threshold":0,"notifications":[{"type":"pagerduty",
		   "url":%q,"routing_key":"${FINFOCUS_NOTIFY_PD_KEY}","severity":"critical"}]}]}}}}`, recv.srv.URL+"/v2/enqueue")
		env := []string{"FINFOCUS_NOTIFY_PD_KEY=" + routingKey}

		baseline := run(t, cfg, env, "--output", "json")
		require.Zero(t, baseline.exitCode, baseline.stderr)
		assert.Contains(t, baseline.stderr, "pass --notify")
		assert.Empty(t, recv.take())

		preview := run(t, cfg, env, "--output", "json", "--notify", "--dry-run")
		require.Zero(t, preview.exitCode, preview.stderr)
		assert.Contains(t, preview.stderr, "trigger, severity critical, routing_key [REDACTED]")
		assert.NotContains(t, preview.stderr+preview.stdout, routingKey)
		assert.Empty(t, recv.take())

		project := t.TempDir()
		require.NoError(
			t,
			os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: test-project\nruntime: go\n"), 0o600),
		)
		var keys []string
		for range 2 {
			sent := run(t, cfg, env, "--output", "json", "--notify", "--debug", "--project-dir", project)
			require.Zero(t, sent.exitCode, sent.stderr)
			assert.True(t, json.Valid([]byte(sent.stdout)))
			assert.NotContains(t, sent.stderr+sent.stdout, routingKey)
			got := recv.take()
			require.Len(t, got, 1)
			assert.Equal(t, "/v2/enqueue", got[0].Path)
			assert.Equal(t, "trigger", got[0].Body["event_action"])
			assert.Equal(t, routingKey, got[0].Body["routing_key"])
			assert.Equal(t, "test-project", got[0].Body["payload"].(map[string]any)["group"])
			keys = append(keys, got[0].Body["dedup_key"].(string))
		}
		assert.Equal(t, keys[0], keys[1])
		assert.True(t, strings.HasPrefix(keys[0], "finfocus:test-project:global:0:actual:"))

		recv.setStatus(http.StatusBadRequest)
		args := []string{"--output", "json", "--exit-on-threshold", "--exit-code", "4"}
		noNotify := run(t, cfg, env, args...)
		failed := run(t, cfg, env, append(args, "--notify")...)
		assert.Equal(t, 4, failed.exitCode, failed.stderr)
		assert.Equal(t, noNotify.stdout, failed.stdout)
		assert.Equal(t, 1, strings.Count(failed.stderr, "warning: pagerduty notification"))
		assert.NotContains(t, failed.stderr, "budget notification not delivered")
		assert.Contains(t, failed.stderr, "status 400")
		assert.NotContains(t, failed.stderr+failed.stdout, routingKey)
		assert.Len(t, recv.take(), 1)
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

func emailBudgetConfig(port int) string {
	return fmt.Sprintf(`{"logging":{"file":""},"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
	  {"threshold":0,"type":"actual","notifications":[{"type":"email",
	    "smtp":{"host":"127.0.0.1","port":%d,"tls":"none",
	            "username":"${FINFOCUS_NOTIFY_SMTP_USER}","password":"${FINFOCUS_NOTIFY_SMTP_PASS}"},
	    "from":"alerts@example.com","to":["owner@example.com"],"cc":["finance@example.com"]}]}
	]}}}}`, port)
}

func newBudgetSMTPRelay(t *testing.T, rejectRecipient bool) (int, <-chan string) {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	messages := make(chan string, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		relay := textproto.NewConn(conn)
		_ = relay.PrintfLine("220 relay ready")
		for {
			line, readErr := relay.ReadLine()
			if readErr != nil {
				return
			}
			verb, _, _ := strings.Cut(line, " ")
			switch verb {
			case "EHLO":
				_ = relay.PrintfLine("250-relay")
				_ = relay.PrintfLine("250 AUTH PLAIN")
			case "AUTH":
				_ = relay.PrintfLine("235 authenticated")
			case "RCPT":
				if rejectRecipient {
					_ = relay.PrintfLine("550 relay-login-secret relay-password-secret")
				} else {
					_ = relay.PrintfLine("250 accepted")
				}
			case "DATA":
				_ = relay.PrintfLine("354 send message")
				data, dataErr := relay.ReadDotBytes()
				if dataErr != nil {
					return
				}
				messages <- string(data)
				_ = relay.PrintfLine("250 queued")
			case "QUIT":
				_ = relay.PrintfLine("221 bye")
				return
			default:
				_ = relay.PrintfLine("250 accepted")
			}
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, messages
}
