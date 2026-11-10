package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

const webPassphraseStreamTimeout = 20 * time.Second

type webStreamEvent struct {
	name string
	data map[string]any
}

// openWebStream opens the overview event stream with a deadline, so a test
// waiting for an event that never comes fails instead of hanging.
func openWebStream(t *testing.T, origin string, cookie *http.Cookie) *bufio.Reader {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), webPassphraseStreamTimeout)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/overview/stream", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return bufio.NewReader(resp.Body)
}

// readWebEventsUntil collects events until stop matches one, the stream
// deadline passes, or the stream ends. The last event is the one that
// matched, when one did.
func readWebEventsUntil(reader *bufio.Reader, stop func(webStreamEvent) bool) ([]webStreamEvent, bool) {
	var events []webStreamEvent
	current := webStreamEvent{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return events, false
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "event: "):
			current.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &current.data)
		case line == "" && current.name != "":
			events = append(events, current)
			if stop(current) {
				return events, true
			}
			current = webStreamEvent{}
		}
	}
}

func asksForPassphrase(event webStreamEvent) bool {
	return event.name == "passphrase_required" ||
		(event.name == "snapshot" && event.data["passphraseRequired"] == true)
}

func isReady(event webStreamEvent) bool { return event.name == "ready" }

func eventNames(events []webStreamEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, event.name)
	}
	return names
}

// currentWebSnapshot opens a fresh stream and returns its first snapshot.
func currentWebSnapshot(t *testing.T, origin string, cookie *http.Cookie) map[string]any {
	t.Helper()
	events, ok := readWebEventsUntil(openWebStream(t, origin, cookie), func(event webStreamEvent) bool {
		return event.name == "snapshot"
	})
	require.True(t, ok, "a new stream must start with a snapshot")
	return events[len(events)-1].data
}

// startupPreviewRunner answers like the Pulumi CLI on an encrypted stack:
// stack export does not decrypt secrets, so it succeeds with any passphrase,
// while preview needs the right one. previewErr, when set, fails every
// preview with that stderr instead. reselectAfterStartup answers the first
// `stack ls` with dev selected and every later one with prod, as if the user
// ran `pulumi stack select prod` once the dashboard started.
type startupPreviewRunner struct {
	state, plan          []byte
	passphrase           string
	previewErr           string
	reselectAfterStartup bool

	mu       sync.Mutex
	previews int
	commands []string
}

func (r *startupPreviewRunner) Run(
	ctx context.Context,
	dir, name string,
	env []string,
	args ...string,
) ([]byte, []byte, error) {
	r.mu.Lock()
	r.commands = append(r.commands, strings.Join(args, " "))
	listed := len(r.ran("stack ls"))
	r.mu.Unlock()
	if r.reselectAfterStartup && strings.Contains(strings.Join(args, " "), "stack ls") && listed > 1 {
		return []byte(`[{"name":"dev"},{"name":"prod","current":true}]`), nil, nil
	}
	if strings.Contains(strings.Join(args, " "), "preview") {
		r.mu.Lock()
		r.previews++
		r.mu.Unlock()
		if r.previewErr != "" {
			return nil, []byte(r.previewErr), errors.New("exit status 1")
		}
		if r.passphrase != "" && !slices.Contains(env, "PULUMI_CONFIG_PASSPHRASE="+r.passphrase) {
			return nil, []byte("error: getting stack configuration: get stack secrets manager: incorrect passphrase"),
				errors.New("exit status 1")
		}
	}
	return (scriptedPulumi{export: r.state, preview: r.plan}).Run(ctx, dir, name, env, args...)
}

// ran lists the Pulumi commands run so far whose arguments contain op. The
// caller holds r.mu.
func (r *startupPreviewRunner) ran(op string) []string {
	var matched []string
	for _, command := range r.commands {
		if strings.Contains(command, op) {
			matched = append(matched, command)
		}
	}
	return matched
}

func (r *startupPreviewRunner) previewCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.previews
}

// pipelineErrorRecorder records the pipeline's fatal error callbacks, which
// the web turns into a failed phase and an error event.
type pipelineErrorRecorder struct {
	mu     sync.Mutex
	phases []int
}

func (r *pipelineErrorRecorder) add(phase int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phases = append(r.phases, phase)
}

func (r *pipelineErrorRecorder) recorded() []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.phases)
}

// startWebPassphraseProject prepares a scripted Pulumi project whose
// Pulumi.yaml is newer than the fixture's deployment, so change detection
// runs a preview at startup, and starts finfocus --web against it.
func startWebPassphraseProject(
	t *testing.T,
	encrypted bool,
	runner *startupPreviewRunner,
	args ...string,
) (string, *http.Cookie, *pipelineErrorRecorder, context.CancelFunc, func() error) {
	t.Helper()
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	writePulumiProject(t, project, encrypted)
	state, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	plan, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	runner.state, runner.plan = state, plan
	t.Cleanup(pulumidetect.SetRunnerForTest(runner))
	errs := &pipelineErrorRecorder{}
	webPipelineHook = func(p *OverviewPipeline) {
		p.cfg.openPlugins = fakeOpenPlugins
		p.cfg.newEngine = fakeNewEngine
		p.cfg.enrichRows = passThroughEnrichment
		published := p.OnError
		p.OnError = func(phase int, err error) {
			errs.add(phase)
			published(phase, err)
		}
	}
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })
	out, cancel, wait := runWebCommand(t, append([]string{"--web", "--no-browser", "--port", "0"}, args...))
	t.Cleanup(cancel)
	origin, cookie := webHTTP(t, awaitURL(t, out))
	return origin, cookie, errs, cancel, wait
}

// TestWebPassphrasePromptNamesTheWorkspaceStack: with no --stack, the
// pipeline resolves the workspace's selected stack before it asks for the
// passphrase, but the prompt is published with an empty stack name, so the
// dialog reads "The stack is passphrase-protected." without naming it.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebPassphrasePromptNamesTheWorkspaceStack(t *testing.T) {
	runner := &startupPreviewRunner{}
	origin, cookie, _, cancel, wait := startWebPassphraseProject(t, true, runner)

	events, asked := readWebEventsUntil(openWebStream(t, origin, cookie), asksForPassphrase)
	require.True(t, asked, "an encrypted stack must ask for its passphrase; events: %v", eventNames(events))
	assert.Equal(t, "dev", events[len(events)-1].data["stack"],
		"the prompt must name the workspace's selected stack (scripted `pulumi stack ls` marks dev current)")

	cancel()
	require.NoError(t, wait())
}

// TestWebStartupPreviewRejectedPassphraseAsksAgain (#2087): stack export does
// not decrypt secrets, so a wrong passphrase gets through state loading and
// fails only at the startup preview. The preview fallback swallows that
// rejection: the session goes on to ready on current state and never asks
// again, where a rejection during state loading reports "Unable to decrypt
// stack" and asks again.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebStartupPreviewRejectedPassphraseAsksAgain(t *testing.T) {
	runner := &startupPreviewRunner{passphrase: "correct-secret"}
	origin, cookie, _, cancel, wait := startWebPassphraseProject(t, true, runner)
	stream := openWebStream(t, origin, cookie)

	events, asked := readWebEventsUntil(stream, asksForPassphrase)
	require.True(t, asked, "an encrypted stack must ask for its passphrase; events: %v", eventNames(events))
	wrong := webAPI(t, origin, cookie, "/api/passphrase", `{"passphrase":"wrong-secret"}`)
	require.Equal(t, http.StatusAccepted, wrong.StatusCode)

	events, asked = readWebEventsUntil(stream, func(event webStreamEvent) bool {
		return asksForPassphrase(event) || isReady(event)
	})
	require.True(t, asked && !isReady(events[len(events)-1]),
		"a passphrase the startup preview rejects must ask again, not finish on current state; events: %v",
		eventNames(events))
	rejection := slices.IndexFunc(events, func(event webStreamEvent) bool { return event.name == "error" })
	require.NotEqual(t, -1, rejection, "the rejection must be reported before the new prompt")
	assert.InDelta(t, OverviewPhaseRunPreview, events[rejection].data["phase"], 0,
		"the rejection belongs to the preview phase that caught it, not to stack loading")

	good := webAPI(t, origin, cookie, "/api/passphrase", `{"passphrase":"correct-secret"}`)
	require.Equal(t, http.StatusAccepted, good.StatusCode)
	events, ready := readWebEventsUntil(stream, isReady)
	require.True(t, ready, "the correct passphrase must finish loading; events: %v", eventNames(events))

	snapshot := currentWebSnapshot(t, origin, cookie)
	assert.Equal(t, false, snapshot["stateOnly"], "the correct passphrase must load the preview")
	assert.Empty(t, snapshot["errors"])
	assert.Equal(t, 2, runner.previewCalls())

	cancel()
	require.NoError(t, wait())
}

// TestWebStartupPreviewConfigErrorIsNotARejectedPassphrase: stackload sorts any
// preview error that mentions a password into the passphrase category, so a
// missing secret config value read as a rejected passphrase and the dashboard
// asked for the correct passphrase again forever instead of loading state.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebStartupPreviewConfigErrorIsNotARejectedPassphrase(t *testing.T) {
	runner := &startupPreviewRunner{
		previewErr: "error: Missing required configuration variable 'app:dbPassword'",
	}
	origin, cookie, _, cancel, wait := startWebPassphraseProject(t, true, runner)
	stream := openWebStream(t, origin, cookie)

	events, asked := readWebEventsUntil(stream, asksForPassphrase)
	require.True(t, asked, "an encrypted stack must ask for its passphrase; events: %v", eventNames(events))
	answer := webAPI(t, origin, cookie, "/api/passphrase", `{"passphrase":"correct-secret"}`)
	require.Equal(t, http.StatusAccepted, answer.StatusCode)

	events, done := readWebEventsUntil(stream, func(event webStreamEvent) bool {
		return asksForPassphrase(event) || isReady(event)
	})
	require.True(t, done && isReady(events[len(events)-1]),
		"a preview config error must fall back to current state, not ask for the passphrase again; events: %v",
		eventNames(events))
	assert.Equal(t, 1, runner.previewCalls())

	snapshot := currentWebSnapshot(t, origin, cookie)
	assert.Equal(t, true, snapshot["stateOnly"], "the session must offer Run preview on current state")
	assert.Empty(t, snapshot["errors"])

	cancel()
	require.NoError(t, wait())
}

// TestWebStartupPreviewFailureIsNotALoadFailure (#2087): when the startup
// preview fails for a reason other than the passphrase, the overview falls
// back to current state as designed, but it also reports the fallback through
// the fatal error callback. The browser receives a failed phase 3 and an
// "Overview loading failed" error, which turn into a red flash once ready
// completes the phase.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebStartupPreviewFailureIsNotALoadFailure(t *testing.T) {
	runner := &startupPreviewRunner{previewErr: "error: failed to load language plugin nodejs"}
	origin, cookie, errs, cancel, wait := startWebPassphraseProject(t, false, runner, "--stack", "dev")

	// The pipeline does not wait for this stream, so it may connect after the
	// events; the recorded callbacks are the deterministic check.
	events, ready := readWebEventsUntil(openWebStream(t, origin, cookie), func(event webStreamEvent) bool {
		return isReady(event) || (event.name == "snapshot" && event.data["ready"] == true)
	})
	require.True(t, ready, "a failed preview must still load current state; events: %v", eventNames(events))
	require.Equal(t, 1, runner.previewCalls(), "the startup preview must have run")
	assert.Empty(t, errs.recorded(),
		"a preview fallback must not reach the fatal error callback (web: failed phase + error event)")
	for _, event := range events {
		assert.NotEqual(t, "error", event.name, "a preview fallback is not a load failure: %v", event.data)
		if event.name == "phase" {
			assert.NotEqual(t, "error", event.data["status"], "no phase may be marked failed: %v", event.data)
		}
	}

	snapshot := currentWebSnapshot(t, origin, cookie)
	assert.Equal(t, true, snapshot["stateOnly"], "the session must offer Run preview on current state")
	assert.Empty(t, snapshot["errors"])

	cancel()
	require.NoError(t, wait())
}
