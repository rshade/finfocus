package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// reserveLoopbackPort binds a loopback TCP port and holds it so a later bind
// of the same port fails. The listener is closed at test cleanup.
func reserveLoopbackPort(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// portOf returns the bound TCP port of ln as a string.
func portOf(t *testing.T, ln net.Listener) string {
	t.Helper()
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return strconv.Itoa(addr.Port)
}

// webCompanionArgs are the root-local flags that are only valid with --web,
// each with a sample value.
func webCompanionArgs() map[string][]string {
	return map[string][]string{
		"port":         {"--port", "8484"},
		"no-browser":   {"--no-browser"},
		"pulumi-json":  {"--pulumi-json", "plan.json"},
		"pulumi-state": {"--pulumi-state", "state.json"},
		"stack":        {"--stack", "dev"},
		"from":         {"--from", "2026-01-01"},
		"to":           {"--to", "2026-01-02"},
		"adapter":      {"--adapter", "aws"},
		"filter":       {"--filter", "type=aws:s3/bucket:Bucket"},
	}
}

// TestWebFlagsAreRootLocal pins --web and its companions as root-local flags:
// present in root.Flags(), absent from root.PersistentFlags(), with no short
// forms (to avoid colliding with ax-go's mounted flags).
//
//nolint:paralleltest // Root builders mutate process-wide config resolution.
func TestWebFlagsAreRootLocal(t *testing.T) {
	root := NewRootCmd("test")
	for _, name := range []string{
		webFlag, "port", "no-browser", "pulumi-json", "pulumi-state",
		"stack", "from", "to", "adapter", "filter",
	} {
		flag := root.Flags().Lookup(name)
		require.NotNil(t, flag, "--%s must be a root-local flag", name)
		assert.Empty(t, flag.Shorthand, "--%s must not have a short form", name)
		assert.Nil(t, root.PersistentFlags().Lookup(name),
			"--%s must NOT be persistent (it would leak into every subcommand)", name)
	}
}

func TestWebRootKeepsProjectDirDefaultDelegation(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Chdir(t.TempDir())
	project := t.TempDir()
	writePulumiProject(t, project, false)
	root := NewRootCmd("test")
	root.SetArgs([]string{"--project-dir", project})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	// Root detection must still delegate to overview. Its cwd auto-detection
	// then fails because cwd is outside a project, as before the extraction.
	err := root.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auto-detecting Pulumi project")
}

// TestWebFlagNotInheritedBySubcommands proves no subcommand accepts --web:
// root-local flags are invisible to children, so the flag is unknown there.
//
// Not parallel: NewRootCmd mutates process-wide config resolution.
func TestWebFlagNotInheritedBySubcommands(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	for _, args := range [][]string{
		{"cost", "actual", "--web"},
		{"overview", "--web"},
	} {
		result := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
		require.NotEqual(t, 0, result.ExitCode, "%v must fail", args)
		assert.Contains(t, string(result.Stderr), "unknown flag: --web", "%v", args)
	}
}

// TestWebCompanionsRequireWeb rejects every companion flag used without
// --web; the error must name --web (a root-local flag would otherwise be
// parsed silently).
//
// Not parallel: NewRootCmd mutates process-wide config resolution; changes the working directory.
func TestWebCompanionsRequireWeb(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	// No Pulumi project here, so without the validation the root would print help.
	t.Chdir(t.TempDir())

	names := make([]string, 0, len(webCompanionArgs()))
	for name := range webCompanionArgs() {
		names = append(names, name)
	}
	//nolint:paralleltest // subtests share the parent's chdir/env state (process-wide).
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			result := axtest.Run(context.Background(), t, NewRootCmd("test"), webCompanionArgs()[name])
			require.Equal(t, int(ax.ExitValidation), result.ExitCode, "--%s without --web must fail validation", name)
			stderr := string(result.Stderr)
			assert.Contains(t, stderr, "--"+name)
			assert.Contains(t, stderr, "--web", "the error must name --web")
		})
	}
}

// TestWebFlagConflictsWithMCP rejects --web with --mcp: the two server entry
// points are mutually exclusive.
//
// Not parallel: NewRootCmd mutates process-wide config resolution.
func TestWebFlagConflictsWithMCP(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Chdir(t.TempDir())

	result := axtest.Run(context.Background(), t, NewRootCmd("test"), []string{"--web", "--mcp"})
	require.Equal(t, int(ax.ExitValidation), result.ExitCode)
	stderr := string(result.Stderr)
	assert.Contains(t, stderr, "--web")
	assert.Contains(t, stderr, "--mcp")
}

// TestWebInvalidDateRange rejects a malformed --from/--to with the validation
// exit code before any server starts.
//
// Not parallel: NewRootCmd mutates process-wide config resolution.
func TestWebInvalidDateRange(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Chdir(t.TempDir())

	for _, args := range [][]string{
		{"--web", "--no-browser", "--from", "not-a-date"},
		{"--web", "--no-browser", "--to", "2026-13-40"},
	} {
		result := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
		require.Equal(t, int(ax.ExitValidation), result.ExitCode, "%v must fail validation", args)
		assert.Contains(t, string(result.Stderr), "invalid date range", "%v", args)
	}
}

// TestWebTerraformStateUnknown proves --terraform-state is not accepted with
// --web: the web UI is Pulumi-only (FR-012a), so the flag is simply unknown.
//
// Not parallel: NewRootCmd mutates process-wide config resolution.
func TestWebTerraformStateUnknown(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	result := axtest.Run(context.Background(), t, NewRootCmd("test"),
		[]string{"--web", "--terraform-state", "state.tfstate"})
	require.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "unknown flag: --terraform-state")
}

// TestWebFlagWhileMCPServing verifies a dispatched MCP tools/call that reaches
// the root with --web still gets errRootNotATool: the MCP branch runs first,
// so --web can never start a server inside a tool call.
//
// Not parallel: swaps the process-wide mcpServeFunc test seam.
func TestWebFlagWhileMCPServing(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")

	var dispatched error
	stubMCPServe(t, func(ctx context.Context, root *cobra.Command, _ string) error {
		call := executeNested(ctx, root, []string{"--web"})
		dispatched = call.err
		return nil
	})

	result := axtest.Run(context.Background(), t, NewRootCmd("v1.2.3"), []string{"--mcp"})
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	require.Error(t, dispatched)
	assert.Contains(t, dispatched.Error(), "not callable as an MCP tool")
}

// TestSubcommandFlagsStillParse guards the flag-redefinition trap: root-local
// --web companions must not shadow or break the same-named flags on
// subcommands (cost --stack, cost history view --plain, overview --stack).
//
// Not parallel: NewRootCmd mutates process-wide config resolution.
func TestSubcommandFlagsStillParse(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	for _, args := range [][]string{
		{"cost", "--stack", "dev", "--help"},
		{"cost", "history", "view", "--plain", "--help"},
		{"overview", "--stack", "dev", "--help"},
	} {
		result := axtest.Run(context.Background(), t, NewRootCmd("test"), args)
		require.Equal(t, 0, result.ExitCode, "%v must still parse: %s", args, result.Stderr)
	}
}

// webOutputBuffer lets tests inspect the printed URL while the CLI is running.
type webOutputBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *webOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *webOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// runWebCommand executes the root with args (which must include --web and
// should include --no-browser) on a cancelable context. It returns the
// captured stdout, a wait function for server exit, and a cancel function.
func runWebCommand(
	t *testing.T, args []string,
) (*webOutputBuffer, context.CancelFunc, func() error) {
	t.Helper()
	root := NewRootCmd("test")
	var out webOutputBuffer
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	ctx, cancelCtx := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()
	return &out, cancelCtx, func() error {
		select {
		case err := <-done:
			return err
		case <-time.After(30 * time.Second):
			require.FailNow(t, "web command did not exit after cancellation")
			return nil
		}
	}
}

// awaitURL polls stdout until the server prints its bootstrap URL.
func awaitURL(t *testing.T, out *webOutputBuffer) string {
	t.Helper()
	var url string
	require.Eventually(t, func() bool {
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(line, "http://127.0.0.1:") && strings.Contains(line, "?token=") {
				url = line
				return true
			}
		}
		return false
	}, 15*time.Second, 10*time.Millisecond, "server must print its bootstrap URL, got: %q", out.String())
	return url
}

// TestWebAutoDetectParity launches finfocus --web from a scripted Pulumi
// project with no source flags and asserts the session resolves the same
// project, stack, and rows as finfocus overview's auto-detect seam, and that
// --project-dir does not change where detection looks.
//
// Not parallel: changes the working directory and the Pulumi runner; swaps the webPipelineHook seam.
func TestWebAutoDetectParity(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)

	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	preview, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: export, preview: preview}))

	// Reference: what finfocus overview loads from this directory.
	ctx := context.Background()
	wantState, wantSteps, wantStack, err := loadOverviewFromAutoDetect(ctx, overviewParams{})
	require.NoError(t, err)
	wantRows, err := engine.MergeResourcesForOverview(ctx, wantState, wantSteps)
	require.NoError(t, err)
	require.NotEmpty(t, wantRows)

	for _, extraArgs := range [][]string{
		{},
		{"--project-dir", t.TempDir()}, // must not steer detection away from the cwd project
	} {
		var (
			gotRows      []engine.OverviewRow
			gotStack     string
			dataReadyHit atomic.Bool
		)
		hookCalled := false
		webPipelineHook = func(p *OverviewPipeline) {
			hookCalled = true
			p.OnDataReady = func(rows []engine.OverviewRow, _ int, stackName string) {
				gotRows = rows
				gotStack = stackName
				dataReadyHit.Store(true)
			}
		}
		t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })

		args := append([]string{"--web", "--no-browser", "--port", "0"}, extraArgs...)
		out, cancel, wait := runWebCommand(t, args)
		awaitURL(t, out)
		require.Eventually(t, dataReadyHit.Load,
			20*time.Second, 10*time.Millisecond, "pipeline must reach data-ready")
		cancel()
		require.NoError(t, wait())

		require.True(t, hookCalled, "web session must build its pipeline")
		assert.Equal(t, wantStack, gotStack)
		assert.Equal(t, wantRows, gotRows,
			"--web with no source flags must load the same rows as finfocus overview")
	}
}

// TestWebNoProjectFailsLikeOverview runs --web from a directory with no
// Pulumi project: it must fail with the same detection error overview gives,
// before any server starts (no URL printed).
//
// Not parallel: changes the working directory.
func TestWebNoProjectFailsLikeOverview(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Chdir(t.TempDir())
	installFakePulumi(t)

	out, cancel, wait := runWebCommand(t, []string{"--web", "--no-browser", "--port", "0"})
	defer cancel()
	err := wait()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find pulumi project")
	assert.NotContains(t, out.String(), "?token=", "no server may start when detection fails")
}

// TestWebPrintsURLOnPinnedPort checks the launch output contract: the
// bootstrap URL with the session token is always printed (FR-002), on the
// requested port when --port pins one (FR-003).
//
// Not parallel: binds a localhost port; NewRootCmd mutates process-wide config resolution.
func TestWebPrintsURLOnPinnedPort(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)
	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	preview, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: export, preview: preview}))

	out, cancel, wait := runWebCommand(t, []string{"--web", "--no-browser", "--port", "0"})
	url := awaitURL(t, out)
	assert.Regexp(t, `^http://127\.0\.0\.1:\d+/\?token=[0-9a-f]{32}$`, url)
	cancel()
	require.NoError(t, wait())
}

// TestWebBusyPinnedPortFails verifies a pinned busy port fails with a clear
// error instead of silently picking another port (FR-003).
//
// Not parallel: binds a localhost port.
func TestWebBusyPinnedPortFails(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")

	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, false)

	ln := reserveLoopbackPort(t)
	out, cancel, wait := runWebCommand(
		t,
		[]string{
			"--web",
			"--no-browser",
			"--port",
			portOf(t, ln),
			"--pulumi-state",
			overviewFixture(t, "state-no-changes.json"),
		},
	)
	defer cancel()
	err := wait()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
	assert.NotContains(t, out.String(), "?token=")
}

func webHTTP(t *testing.T, bootstrap string) (string, *http.Cookie) {
	t.Helper()
	parsed, err := url.Parse(bootstrap)
	require.NoError(t, err)
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(bootstrap)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Len(t, resp.Cookies(), 1)
	return parsed.Scheme + "://" + parsed.Host, resp.Cookies()[0]
}
func webAPI(t *testing.T, origin string, cookie *http.Cookie, path, body string) *http.Response {
	t.Helper()
	method := http.MethodGet
	if body != "" {
		method = http.MethodPost
	}
	req, err := http.NewRequest(method, origin+path, strings.NewReader(body))
	require.NoError(t, err)
	req.AddCookie(cookie)
	if method == http.MethodPost {
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}
func webEvent(t *testing.T, reader *bufio.Reader) (string, map[string]any) {
	t.Helper()
	name := ""
	var data map[string]any
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			name = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data))
		}
		if line == "" && name != "" {
			return name, data
		}
	}
}

// Root builders, cwd and pipeline hooks affect process-wide state.
func TestWebPipelineStreamsFixtureAndStopsWithOpenStream(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	t.Chdir(t.TempDir())
	gate := make(chan struct{})
	webPipelineHook = func(p *OverviewPipeline) {
		p.cfg.openPlugins = fakeOpenPlugins
		p.cfg.newEngine = fakeNewEngine
		p.cfg.enrichRows = passThroughEnrichment
		original := p.OnPhase
		p.OnPhase = func(phase int, name string) {
			if phase == 1 {
				<-gate
			}
			original(phase, name)
		}
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })
	out, cancel, wait := runWebCommand(
		t,
		[]string{"--web", "--no-browser", "--pulumi-json", overviewFixture(t, "plan-mixed-changes.json")},
	)
	defer cancel()
	origin, cookie := webHTTP(t, awaitURL(t, out))
	response := webAPI(t, origin, cookie, "/api/overview/stream", "")
	reader := bufio.NewReader(response.Body)
	name, _ := webEvent(t, reader)
	assert.Equal(t, "snapshot", name)
	close(gate)
	var phases []int
	rows := 0
	for {
		event, data := webEvent(t, reader)
		if event == "phase" && data["status"] == "active" {
			phases = append(phases, int(data["phase"].(float64)))
		}
		if event == "row" {
			rows++
		}
		if event == "ready" {
			break
		}
	}
	assert.Equal(t, []int{1, 2, 3, 4, 5, 6}, phases)
	assert.Positive(t, rows)
	query := webAPI(t, origin, cookie, "/api/overview/query", `{}`)
	require.Equal(t, http.StatusOK, query.StatusCode)
	cancel()
	start := time.Now()
	require.NoError(t, wait())
	assert.Less(t, time.Since(start), 2*time.Second)
}

type encryptedWebRunner struct{ state []byte }

func (r encryptedWebRunner) Run(
	ctx context.Context,
	dir, name string,
	env []string,
	args ...string,
) ([]byte, []byte, error) {
	if strings.Contains(strings.Join(args, " "), "stack export") &&
		slices.Contains(env, "PULUMI_CONFIG_PASSPHRASE=bad-secret") {
		return nil, []byte("incorrect passphrase bad-secret"), errors.New("exit status 1")
	}
	return (scriptedPulumi{export: r.state}).Run(ctx, dir, name, env, args...)
}

// Root builders, cwd and Pulumi runner affect process-wide state.
func TestWebEncryptedStackRetriesWithoutEcho(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	project := t.TempDir()
	t.Chdir(project)
	writePulumiProject(t, project, true)
	installFakePulumi(t)
	state, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	t.Cleanup(pulumidetect.SetRunnerForTest(encryptedWebRunner{state: state}))
	webPipelineHook = func(p *OverviewPipeline) {
		p.cfg.openPlugins = fakeOpenPlugins
		p.cfg.newEngine = fakeNewEngine
		p.cfg.enrichRows = passThroughEnrichment
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })
	out, cancel, wait := runWebCommand(t, []string{"--web", "--no-browser", "--stack", "dev"})
	defer cancel()
	origin, cookie := webHTTP(t, awaitURL(t, out))
	stream := webAPI(t, origin, cookie, "/api/overview/stream", "")
	reader := bufio.NewReader(stream.Body)
	name, snapshot := webEvent(t, reader)
	assert.Equal(t, "snapshot", name)
	if snapshot["passphraseRequired"] != true {
		for {
			name, _ = webEvent(t, reader)
			if name == "passphrase_required" {
				break
			}
		}
	}
	wrong := webAPI(t, origin, cookie, "/api/passphrase", `{"passphrase":"bad-secret"}`)
	require.Equal(t, http.StatusAccepted, wrong.StatusCode)
	seenError := false
	for {
		event, data := webEvent(t, reader)
		encoded, marshalErr := json.Marshal(data)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(encoded), "bad-secret")
		if event == "error" {
			seenError = true
		}
		if event == "passphrase_required" {
			break
		}
	}
	assert.True(t, seenError)
	good := webAPI(t, origin, cookie, "/api/passphrase", `{"passphrase":"correct-secret"}`)
	require.Equal(t, http.StatusAccepted, good.StatusCode)
	for {
		event, data := webEvent(t, reader)
		encoded, marshalErr := json.Marshal(data)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(encoded), "correct-secret")
		if event == "ready" {
			break
		}
	}
	cancel()
	require.NoError(t, wait())
	assert.NotContains(t, out.String(), "bad-secret")
	assert.NotContains(t, out.String(), "correct-secret")
}
