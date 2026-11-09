package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/webui"
)

func TestNormalizeStackList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{"none", nil, nil},
		{"empty slice", []string{}, nil},
		{"lone empty value is no stack", []string{""}, nil},
		{"lone blank value is no stack", []string{"  "}, nil},
		{"one name", []string{"dev"}, []string{"dev"}},
		{"comma separated", []string{"a,b"}, []string{"a", "b"}},
		{"repeats", []string{"a", "b"}, []string{"a", "b"}},
		{"repeats and commas", []string{"a,b", "c"}, []string{"a", "b", "c"}},
		{"whitespace is trimmed", []string{" a , b "}, []string{"a", "b"}},
		{"duplicates keep first-seen order", []string{"b,a,b,c,a"}, []string{"b", "a", "c"}},
		{"duplicates across repeats", []string{"a", "b", "a"}, []string{"a", "b"}},
		{"duplicates that leave one name", []string{"a,a"}, []string{"a"}},
		{"single qualified name is unchanged", []string{"org/proj/dev"}, []string{"org/proj/dev"}},
		{"qualified name repeated leaves one", []string{"org/proj/dev,org/proj/dev"}, []string{"org/proj/dev"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeStackList(tt.values)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeStackListRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  []string
		message string
	}{
		{"empty element in the middle", []string{"a,,b"}, "empty"},
		{"trailing comma", []string{"a,"}, "empty"},
		{"leading comma", []string{",a"}, "empty"},
		{"only commas", []string{","}, "empty"},
		{"empty repeat after a name", []string{"a", ""}, "empty"},
		{"empty repeat before a name", []string{"", "a"}, "empty"},
		{"blank element", []string{"a, ,b"}, "empty"},
		{"name with a slash beside another", []string{"a,b/c"}, `"b/c"`},
		{"name with a space beside another", []string{"a,bad name"}, `"bad name"`},
		{"name too long beside another", []string{"a," + strings.Repeat("x", 101)}, "not a valid stack name"},
		{"dot segment beside another", []string{"a,.."}, `".."`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeStackList(tt.values)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestStackFlagName(t *testing.T) {
	t.Parallel()

	stringFlag := func(cmd *cobra.Command) { cmd.Flags().String("stack", "", "") }
	listFlag := func(cmd *cobra.Command) { cmd.Flags().StringArray("stack", nil, "") }
	tests := []struct {
		name    string
		declare func(*cobra.Command)
		value   string
		want    string
	}{
		{name: "string flag", declare: stringFlag, value: "dev", want: "dev"},
		{name: "unset string flag", declare: stringFlag},
		{name: "list flag with one name", declare: listFlag, value: "org/proj/dev", want: "org/proj/dev"},
		{name: "unset list flag", declare: listFlag},
		{name: "list flag with several names has no single name", declare: listFlag, value: "a,b"},
		{name: "no flag at all", declare: func(*cobra.Command) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{}
			tt.declare(cmd)
			if tt.value != "" {
				require.NoError(t, cmd.Flags().Set("stack", tt.value))
			}
			assert.Equal(t, tt.want, stackFlagName(cmd))
		})
	}
	assert.Empty(t, stackFlagName(nil))
}

// stackListRunner answers `pulumi stack ls --json` with the given stacks and
// counts the calls, so a test can prove validation stopped before asking.
type stackListRunner struct {
	names []string
	err   error
	calls *atomic.Int32
}

func (r stackListRunner) Run(_ context.Context, _, name string, _ []string, args ...string) ([]byte, []byte, error) {
	r.calls.Add(1)
	if name != "pulumi" || strings.Join(args, " ") != "stack ls --json" {
		return nil, []byte(strings.Join(args, " ")), errors.New("unexpected command")
	}
	if r.err != nil {
		return nil, []byte("not logged in"), r.err
	}
	entries := make([]string, 0, len(r.names))
	for _, n := range r.names {
		entries = append(entries, `{"name":"`+n+`","current":false}`)
	}
	return []byte("[" + strings.Join(entries, ",") + "]"), nil, nil
}

type stackListProject int

const (
	pulumiProject stackListProject = iota
	terraformProject
	noProject
)

func enterStackListProject(t *testing.T, kind stackListProject, r stackListRunner) {
	t.Helper()
	dir := t.TempDir()
	switch kind {
	case pulumiProject:
		writePulumiProject(t, dir, false)
	case terraformProject:
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# Terraform root\n"), 0o600))
	case noProject:
		// An empty directory: detection finds neither Pulumi.yaml nor *.tf.
	}
	t.Chdir(dir)
	installFakePulumi(t)
	t.Cleanup(pulumidetect.SetRunnerForTest(r))
}

//nolint:paralleltest // changes cwd, PATH, and the Pulumi runner
func TestValidateStackList(t *testing.T) {
	tests := []struct {
		name       string
		project    stackListProject
		flags      webFlagValues
		names      []string
		available  []string
		listErr    error
		wantExit   int
		wantInErr  []string
		wantAsked  bool
		wantNoCall bool
	}{
		{
			name: "every name is a stack of the project", project: pulumiProject,
			names: []string{"dev", "prod"}, available: []string{"dev", "prod", "test"}, wantAsked: true,
		},
		{
			name: "unknown name lists the available stacks", project: pulumiProject,
			names: []string{"dev", "nope"}, available: []string{"dev", "prod"}, wantAsked: true,
			wantExit: ax.ExitValidation, wantInErr: []string{"nope", "dev, prod"},
		},
		{
			name: "pulumi-json is rejected before asking Pulumi", project: pulumiProject,
			flags: webFlagValues{pulumiJSON: "plan.json"}, names: []string{"dev", "prod"},
			available: []string{"dev", "prod"}, wantExit: ax.ExitValidation,
			wantInErr: []string{"Pulumi project's own stacks"}, wantNoCall: true,
		},
		{
			name: "pulumi-state is rejected before asking Pulumi", project: pulumiProject,
			flags: webFlagValues{pulumiState: "state.json"}, names: []string{"dev", "prod"},
			available: []string{"dev", "prod"}, wantExit: ax.ExitValidation,
			wantInErr: []string{"Pulumi project's own stacks"}, wantNoCall: true,
		},
		{
			name: "terraform project is rejected", project: terraformProject,
			names: []string{"dev", "prod"}, available: []string{"dev", "prod"}, wantExit: ax.ExitValidation,
			wantInErr: []string{"Pulumi project's own stacks"}, wantNoCall: true,
		},
		{
			name: "no project is a validation error", project: noProject,
			names: []string{"dev", "prod"}, wantExit: ax.ExitValidation, wantNoCall: true,
		},
		{
			name: "a failing stack listing is not a validation error", project: pulumiProject,
			names: []string{"dev", "prod"}, listErr: errors.New("exit status 255"), wantAsked: true,
			wantExit: ax.ExitInternal, wantInErr: []string{"listing Pulumi stacks"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			enterStackListProject(t, tt.project, stackListRunner{names: tt.available, err: tt.listErr, calls: &calls})
			flags := tt.flags

			err := validateStackList(context.Background(), &flags, tt.names)

			if tt.wantExit == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantExit, ax.ErrorExitCode(err), err.Error())
				for _, fragment := range tt.wantInErr {
					assert.Contains(t, err.Error(), fragment)
				}
			}
			if tt.wantNoCall {
				assert.Zero(t, calls.Load(), "Pulumi must not be asked")
			}
			if tt.wantAsked {
				assert.Positive(t, calls.Load())
			}
		})
	}
}

func TestStackFlagNameReadsTheRealWebFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"one name", []string{"--stack", "dev"}, "dev"},
		{"one name with equals", []string{"--stack=org/proj/dev"}, "org/proj/dev"},
		{"unset", nil, ""},
		{"a list names no single stack", []string{"--stack=dev,prod"}, ""},
		{"repeats name no single stack", []string{"--stack", "dev", "--stack", "prod"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := &cobra.Command{Use: "test"}
			registerWebFlags(cmd, &webFlagValues{})
			require.NoError(t, cmd.ParseFlags(tt.args))
			assert.Equal(t, tt.want, stackFlagName(cmd))
		})
	}
}

// pipelineRuns counts loader runs per stack, keyed by the stack the pipeline
// was built for.
type pipelineRuns struct{ counts sync.Map }

func (r *pipelineRuns) count(stack string) int32 {
	value, ok := r.counts.Load(stack)
	if !ok {
		return 0
	}
	return value.(*atomic.Int32).Load()
}

func (r *pipelineRuns) hook(p *OverviewPipeline) {
	p.cfg.openPlugins = fakeOpenPlugins
	p.cfg.newEngine = fakeNewEngine
	p.cfg.enrichRows = passThroughEnrichment
	stack := p.cfg.params.stack
	load := fakePipelineLoader(fakePipelineRows())
	p.cfg.loader = func(ctx context.Context, pipe *OverviewPipeline) (*OverviewPipelineData, error) {
		counter, _ := r.counts.LoadOrStore(stack, &atomic.Int32{})
		counter.(*atomic.Int32).Add(1)
		return load(ctx, pipe)
	}
}

func startMultiStackDashboard(
	t *testing.T, stacks string, hook func(*OverviewPipeline),
) (string, *http.Cookie, func() error) {
	t.Helper()
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Setenv("FINFOCUS_SKIP_MIGRATION_CHECK", "1")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	enterStackListProject(t, pulumiProject, stackListRunner{
		names: strings.Split(stacks, ","), calls: &atomic.Int32{},
	})
	webPipelineHook = hook
	t.Cleanup(func() { webPipelineHook = func(*OverviewPipeline) {} })

	out, cancel, wait := runWebCommand(t, []string{"--web", "--no-browser", "--stack", stacks})
	t.Cleanup(cancel)
	origin, cookie := webHTTP(t, awaitURL(t, out))
	return origin, cookie, func() error { cancel(); return wait() }
}

func fetchStackStates(t *testing.T, origin string, cookie *http.Cookie) map[string]webui.StackState {
	t.Helper()
	resp := webAPI(t, origin, cookie, "/api/stacks", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var parsed struct {
		Stacks []webui.StackEntry `json:"stacks"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	states := make(map[string]webui.StackState, len(parsed.Stacks))
	for _, entry := range parsed.Stacks {
		states[entry.Name] = entry.State
	}
	return states
}

//nolint:paralleltest // builds the root command, changes cwd, and sets the web pipeline hook
func TestWebStacks_OnlyTheActivatedStackRuns(t *testing.T) {
	runs := &pipelineRuns{}
	origin, cookie, stop := startMultiStackDashboard(t, "dev,prod,test", runs.hook)

	assert.Equal(t, map[string]webui.StackState{
		"dev": webui.StackStateIdle, "prod": webui.StackStateIdle, "test": webui.StackStateIdle,
	}, fetchStackStates(t, origin, cookie), "starting the dashboard must start no stack")
	assert.Zero(t, runs.count("dev")+runs.count("prod")+runs.count("test"))

	for range 3 {
		resp := webAPI(t, origin, cookie, "/api/stacks/prod/activate", `{}`)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	require.Eventually(t, func() bool {
		return fetchStackStates(t, origin, cookie)["prod"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond, "prod must reach ready")

	assert.Equal(t, int32(1), runs.count("prod"), "activating again must not run the pipeline again")
	assert.Zero(t, runs.count("dev"))
	assert.Zero(t, runs.count("test"))
	states := fetchStackStates(t, origin, cookie)
	assert.Equal(t, webui.StackStateIdle, states["dev"])
	assert.Equal(t, webui.StackStateIdle, states["test"])

	for range 2 {
		query := webAPI(t, origin, cookie, "/s/prod/api/overview/query", `{}`)
		require.Equal(t, http.StatusOK, query.StatusCode)
		body, err := io.ReadAll(query.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs",
			"a reselected stack serves the rows it already loaded")
	}
	assert.Equal(t, int32(1), runs.count("prod"))

	start := time.Now()
	require.NoError(t, stop(), "shutdown with never-started stacks must complete")
	assert.Less(t, time.Since(start), 5*time.Second)
}

//nolint:paralleltest // builds the root command, changes cwd, and sets the web pipeline hook
func TestWebStacks_EachStackLoadsOnItsOwnActivation(t *testing.T) {
	runs := &pipelineRuns{}
	origin, cookie, stop := startMultiStackDashboard(t, "dev,prod", runs.hook)

	for _, name := range []string{"dev", "prod"} {
		resp := webAPI(t, origin, cookie, "/api/stacks/"+name+"/activate", `{}`)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	require.Eventually(t, func() bool {
		states := fetchStackStates(t, origin, cookie)
		return states["dev"] == webui.StackStateReady && states["prod"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond)

	assert.Equal(t, int32(1), runs.count("dev"))
	assert.Equal(t, int32(1), runs.count("prod"))
	require.NoError(t, stop())
}

// stackOwnRowsHook gives each stack's pipeline rows under its own stack name,
// as the CLI would load for that --stack.
func stackOwnRowsHook(p *OverviewPipeline) {
	p.cfg.openPlugins = fakeOpenPlugins
	p.cfg.newEngine = fakeNewEngine
	p.cfg.enrichRows = passThroughEnrichment
	p.cfg.loader = fakePipelineLoader([]engine.OverviewRow{{
		URN:    "urn:pulumi:" + p.cfg.params.stack + "::app::aws:s3/bucket:Bucket::logs",
		Type:   "aws:s3/bucket:Bucket",
		Status: engine.StatusActive,
	}})
}

// Each tab serves the data of the stack it names, through the same pipeline
// the single-stack dashboard and the CLI build for that --stack.
//
//nolint:paralleltest // builds the root command, changes cwd, and sets the web pipeline hook
func TestWebStacks_EachStackServesItsOwnData(t *testing.T) {
	origin, cookie, stop := startMultiStackDashboard(t, "dev,prod", stackOwnRowsHook)

	for _, name := range []string{"dev", "prod"} {
		resp := webAPI(t, origin, cookie, "/api/stacks/"+name+"/activate", `{}`)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	require.Eventually(t, func() bool {
		states := fetchStackStates(t, origin, cookie)
		return states["dev"] == webui.StackStateReady && states["prod"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond)

	for name, other := range map[string]string{"dev": "prod", "prod": "dev"} {
		query := webAPI(t, origin, cookie, "/s/"+name+"/api/overview/query", `{}`)
		require.Equal(t, http.StatusOK, query.StatusCode)
		body, err := io.ReadAll(query.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "urn:pulumi:"+name+"::app::", name)
		assert.NotContains(t, string(body), "urn:pulumi:"+other+"::app::", name)
	}
	require.NoError(t, stop())
}

//nolint:paralleltest // builds web stacks that read process-wide config
func TestStartedRowsEnrichedCountsOnlyStartedStacks(t *testing.T) {
	stack, _ := newCountingStack(t)
	started, _ := newCountingStack(t)
	started.start()
	select {
	case <-started.pipeDone:
	case <-time.After(10 * time.Second):
		require.Fail(t, "started stack never finished")
	}
	stack.pipe.result.RowsEnriched = 7
	started.pipe.result.RowsEnriched = 5

	assert.Equal(t, 5, startedRowsEnriched([]*webStack{stack, started}))
	assert.Zero(t, startedRowsEnriched(nil))
}

const rawFailure = "dial tcp 10.0.0.7: PULUMI_ACCESS_TOKEN=pul-secret rejected"

// failingFor makes the named stack's loader fail at start with an error whose
// text must never reach the browser.
func failingFor(bad string, healthy func(*OverviewPipeline)) func(*OverviewPipeline) {
	return func(p *OverviewPipeline) {
		healthy(p)
		if p.cfg.params.stack != bad {
			return
		}
		p.cfg.loader = func(context.Context, *OverviewPipeline) (*OverviewPipelineData, error) {
			return nil, errors.New(rawFailure)
		}
	}
}

//nolint:paralleltest // builds the root command, changes cwd, and sets the web pipeline hook
func TestWebStacks_AFailingStackDoesNotBreakTheOthers(t *testing.T) {
	runs := &pipelineRuns{}
	origin, cookie, stop := startMultiStackDashboard(t, "bad,good", failingFor("bad", runs.hook))

	for _, name := range []string{"bad", "good"} {
		resp := webAPI(t, origin, cookie, "/api/stacks/"+name+"/activate", `{}`)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	require.Eventually(t, func() bool {
		states := fetchStackStates(t, origin, cookie)
		return states["bad"] == webui.StackStateFailed && states["good"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond, "bad must fail and good must still load")

	stream := webAPI(t, origin, cookie, "/s/bad/api/overview/stream", "")
	name, snapshot := webEvent(t, bufio.NewReader(stream.Body))
	require.Equal(t, "snapshot", name)
	encoded, err := json.Marshal(snapshot)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "Overview loading failed", "the stack reports the safe label")
	assert.NotContains(t, string(encoded), "pul-secret")
	assert.NotContains(t, string(encoded), "10.0.0.7")

	query := webAPI(t, origin, cookie, "/s/good/api/overview/query", `{}`)
	require.Equal(t, http.StatusOK, query.StatusCode)
	body, err := io.ReadAll(query.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs")
	assert.Equal(t, int32(1), runs.count("good"))

	assert.Equal(t, webui.StackStateFailed, fetchStackStates(t, origin, cookie)["bad"], "failed is final")
	require.NoError(t, stop())
}

// encryptedStackHook gives a stack a loader that asks for its passphrase and
// rejects any other, the way a real encrypted stack does: the pipeline's own
// retry loop then records the failure and prompts again.
func encryptedStackHook(secrets map[string]string, rejections *sync.Map) func(*OverviewPipeline) {
	return func(p *OverviewPipeline) {
		p.cfg.openPlugins = fakeOpenPlugins
		p.cfg.newEngine = fakeNewEngine
		p.cfg.enrichRows = passThroughEnrichment
		stack := p.cfg.params.stack
		ready := fakePipelineLoader(fakePipelineRows())
		p.cfg.loader = func(ctx context.Context, pipe *OverviewPipeline) (*OverviewPipelineData, error) {
			if pipe.Passphrase() == nil {
				pipe.OnPassphraseRequired()
				select {
				case pw := <-pipe.cfg.passphrases:
					pipe.mu.Lock()
					pipe.passphrase = &pw
					pipe.mu.Unlock()
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			if *pipe.Passphrase() != secrets[stack] {
				counter, _ := rejections.LoadOrStore(stack, &atomic.Int32{})
				counter.(*atomic.Int32).Add(1)
				return nil, errors.New("unable to decrypt stack: incorrect passphrase")
			}
			return ready(ctx, pipe)
		}
	}
}

func rejectionCount(rejections *sync.Map, stack string) int32 {
	counter, ok := rejections.Load(stack)
	if !ok {
		return 0
	}
	return counter.(*atomic.Int32).Load()
}

//nolint:paralleltest // builds the root command, changes cwd, and sets the web pipeline hook
func TestWebStacks_EachEncryptedStackAsksForItsOwnPassphrase(t *testing.T) {
	var rejections sync.Map
	hook := encryptedStackHook(map[string]string{"dev": "dev-secret", "prod": "prod-secret"}, &rejections)
	origin, cookie, stop := startMultiStackDashboard(t, "dev,prod", hook)

	for _, name := range []string{"dev", "prod"} {
		resp := webAPI(t, origin, cookie, "/api/stacks/"+name+"/activate", `{}`)
		require.Equal(t, http.StatusAccepted, resp.StatusCode)
	}
	require.Eventually(t, func() bool {
		states := fetchStackStates(t, origin, cookie)
		return states["dev"] == webui.StackStateLocked && states["prod"] == webui.StackStateLocked
	}, 15*time.Second, 20*time.Millisecond, "both stacks must prompt")

	wrong := webAPI(t, origin, cookie, "/s/prod/api/passphrase", `{"passphrase":"wrong"}`)
	require.Equal(t, http.StatusAccepted, wrong.StatusCode)
	assert.Never(t, func() bool {
		return fetchStackStates(t, origin, cookie)["prod"] == webui.StackStateFailed
	}, 300*time.Millisecond, 10*time.Millisecond, "a rejected passphrase is retryable, never failed")
	require.Eventually(t, func() bool {
		return rejectionCount(&rejections, "prod") == 1 &&
			fetchStackStates(t, origin, cookie)["prod"] == webui.StackStateLocked
	}, 15*time.Second, 10*time.Millisecond, "prod must prompt again after the rejection")
	assert.Equal(t, webui.StackStateLocked, fetchStackStates(t, origin, cookie)["dev"], "dev must keep waiting")
	assert.Zero(t, rejectionCount(&rejections, "dev"), "prod's attempt never reached dev")

	right := webAPI(t, origin, cookie, "/s/prod/api/passphrase", `{"passphrase":"prod-secret"}`)
	require.Equal(t, http.StatusAccepted, right.StatusCode)
	require.Eventually(t, func() bool {
		return fetchStackStates(t, origin, cookie)["prod"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond, "the correct passphrase unlocks prod")
	assert.Equal(t, webui.StackStateLocked, fetchStackStates(t, origin, cookie)["dev"],
		"unlocking prod leaves dev locked")

	devRight := webAPI(t, origin, cookie, "/s/dev/api/passphrase", `{"passphrase":"dev-secret"}`)
	require.Equal(t, http.StatusAccepted, devRight.StatusCode)
	require.Eventually(t, func() bool {
		return fetchStackStates(t, origin, cookie)["dev"] == webui.StackStateReady
	}, 15*time.Second, 20*time.Millisecond)
	require.NoError(t, stop())
}
