package integration_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/test/integration/helpers"
)

const escRedactionSecret = "do-not-deliver-7f3a"

func TestESCRedaction(t *testing.T) {
	binPath := recorderBinary(t)
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("Plugin binary not found at %s. Run 'make build-recorder' first.", binPath)
	}
	home := t.TempDir()
	installRecorder(t, home, binPath)
	logPath := filepath.Join(home, "finfocus.log")
	recordDir := filepath.Join(home, "recordings")
	require.NoError(t, os.MkdirAll(recordDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"),
		[]byte(`{"logging":{"level":"debug","format":"json","file":"`+logPath+`"}}`), 0o600))

	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/redact": {
			ETag: "rev-1",
			Variables: map[string]esctest.Variable{
				"UNRELATED_SECRET": {Value: escRedactionSecret, Secret: true},
			},
		},
	})
	t.Cleanup(api.Close)

	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: demo\nruntime: yaml\n"), 0o600),
	)
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.dev.yaml"),
		[]byte("environment:\n  - acme/billing/redact\n"), 0o600))
	plan := filepath.Join(project, "plan.json")
	planBody, err := os.ReadFile(simplePlanFile(t))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(plan, planBody, 0o600))

	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
	t.Setenv("PULUMI_BACKEND_URL", api.URL)
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", recordDir)
	t.Chdir(project)
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	helper := helpers.NewCLIHelper(t)
	stdout, err := helper.Execute(
		"cost",
		"projected",
		"--debug",
		"--format",
		"json",
		"--pulumi-json",
		plan,
		"--stack",
		"dev",
	)
	require.NoError(t, err, "stderr: %s", helper.Stderr())

	opensBeforeWeb := api.Opens("acme", "billing", "redact")
	settingsBody, webOut, webErr := fetchRecorderSettings(t, plan)
	assert.Greater(t, api.Opens("acme", "billing", "redact"), opensBeforeWeb,
		"the web session must open the stack environment")
	scanned := stdout + helper.Stderr() + webOut + webErr + settingsBody + readAllFiles(t, home)
	assert.NotContains(t, scanned, escRedactionSecret)
}

func installRecorder(t *testing.T, home, binPath string) {
	t.Helper()
	dir := filepath.Join(home, "plugins", "recorder", "0.1.0")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	source, err := os.Open(binPath)
	require.NoError(t, err)
	defer source.Close()
	target, err := os.OpenFile(
		filepath.Join(dir, "finfocus-plugin-recorder"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o755,
	)
	require.NoError(t, err)
	_, err = io.Copy(target, source)
	require.NoError(t, target.Close())
	require.NoError(t, err)
	manifest, err := os.ReadFile(filepath.Join(repoRoot(t), "plugins", "recorder", "plugin.manifest.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), manifest, 0o600))
}

func fetchRecorderSettings(t *testing.T, plan string) (string, string, string) {
	t.Helper()
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errBuf lockedBuffer
	cmd := cli.NewRootCmd("test")
	cmd.SetArgs([]string{"--web", "--no-browser", "--port", port, "--pulumi-json", plan, "--stack", "dev"})
	config.ResetGlobalConfigForTest()
	done := make(chan int, 1)
	go func() {
		done <- ax.Execute(ctx, cmd, ax.WithStdout(&out), ax.WithStderr(&errBuf), ax.WithVersion("test"))
	}()

	sessionURL := waitForURL(t, &out, &errBuf, done)
	body := getPluginSettings(t, sessionURL)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("web server did not stop")
	}
	return body, out.String(), errBuf.String()
}

func waitForURL(t *testing.T, out, errBuf *lockedBuffer, done <-chan int) string {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.Contains(line, "token=") {
				return strings.TrimSpace(line)
			}
		}
		select {
		case code := <-done:
			require.FailNowf(
				t, "web server exited", "code %d\nstdout: %s\nstderr: %s",
				code, out.String(), errBuf.String(),
			)
		case <-time.After(50 * time.Millisecond):
		}
	}
	require.FailNowf(t, "timed out waiting for the web URL", "stdout: %s\nstderr: %s", out.String(), errBuf.String())
	return ""
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func readAllFiles(t *testing.T, root string) string {
	t.Helper()
	var buf strings.Builder
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		buf.Write(body)
		return nil
	})
	require.NoError(t, err)
	return buf.String()
}

func getPluginSettings(t *testing.T, sessionURL string) string {
	t.Helper()
	parsed, err := url.Parse(sessionURL)
	require.NoError(t, err)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	bootstrap, err := client.Get(sessionURL)
	require.NoError(t, err)
	require.NoError(t, bootstrap.Body.Close())
	settings, err := client.Get(parsed.Scheme + "://" + parsed.Host + "/api/plugins/recorder/settings")
	require.NoError(t, err)
	defer settings.Body.Close()
	require.Equal(t, http.StatusOK, settings.StatusCode)
	payload, err := io.ReadAll(bufio.NewReader(settings.Body))
	require.NoError(t, err)
	return string(payload)
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}

func simplePlanFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "plans", "aws-simple-plan.json")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(filepath.Dir(recorderBinary(t)), "..")
}
