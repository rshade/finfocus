package pluginhost_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

const (
	credentialName     = "token"
	credentialVariable = "RECORDER_TOKEN"
	noCredential       = "no credential"
	secretA            = "test-secret-value-a"
	secretB            = "test-secret-value-b"
	secretMultiline    = "test-secret-line-1\ntest-secret-line-2"
)

// credentialPlugin answers Supports with the credential its call carried. With
// echo it repeats the credential in an error, as a careless plugin would.
type credentialPlugin struct {
	*pluginsdk.BasePlugin

	echo bool
}

func (p *credentialPlugin) Supports(
	ctx context.Context, _ *pbc.SupportsRequest,
) (*pbc.SupportsResponse, error) {
	creds, err := pluginsdk.ExtractCredentials(ctx)
	if err != nil {
		return nil, err
	}
	value, ok := creds.Get(credentialName)
	if !ok {
		value = noCredential
	}
	if p.echo {
		return nil, status.Errorf(codes.PermissionDenied, "backend refused %s", value)
	}
	return &pbc.SupportsResponse{Supported: true, Reason: value}, nil
}

// optedInPlugin advertises that it reads per-request credentials.
type optedInPlugin struct{ *credentialPlugin }

func (optedInPlugin) ConsumesPerRequestCredentials() {}

// launchSpec describes the plugin of one launch.
type launchSpec struct {
	optOut   bool
	echo     bool
	infoFail bool
}

// credentialLauncher serves a pluginsdk plugin for every Start, so credentials
// travel the same wire path as in a plugin process. specs holds one entry per
// launch; the last one repeats.
type credentialLauncher struct {
	t     *testing.T
	specs []launchSpec
	// gate, when set, holds every Start until it closes.
	gate chan struct{}

	mu        sync.Mutex
	processes []*fakeProcess
}

func (l *credentialLauncher) Start(
	ctx context.Context, _ string, _ ...string,
) (*grpc.ClientConn, func() error, error) {
	if l.gate != nil {
		<-l.gate
	}
	l.mu.Lock()
	spec := launchSpec{}
	if len(l.specs) > 0 {
		spec = l.specs[min(len(l.processes), len(l.specs)-1)]
	}
	delivered, _ := pluginhost.DeliveredEnvFromContext(ctx)
	process := &fakeProcess{ctx: ctx, env: append([]string(nil), delivered.Pairs...)}
	l.processes = append(l.processes, process)
	l.mu.Unlock()

	plugin := &credentialPlugin{BasePlugin: pluginsdk.NewBasePlugin(poolPlugin), echo: spec.echo}
	var served pluginsdk.Plugin = optedInPlugin{plugin}
	if spec.optOut {
		served = plugin
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	quiet := zerolog.Nop()
	serveCtx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = pluginsdk.Serve(serveCtx, pluginsdk.ServeConfig{
			Plugin: served, Listener: listener, Logger: &quiet,
			PluginInfo: pluginsdk.NewPluginInfo(poolPlugin, "v1.0.0"),
			UnaryInterceptors: []grpc.UnaryServerInterceptor{func(
				ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
			) (any, error) {
				if spec.infoFail && strings.HasSuffix(info.FullMethod, "/GetPluginInfo") {
					return nil, status.Error(codes.Unavailable, "not now")
				}
				return handler(ctx, req)
			}},
		})
	}()
	conn, err := grpc.NewClient(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(pluginhost.TraceInterceptor()))
	if err != nil {
		stop()
		return nil, nil, err
	}
	var once sync.Once
	closeFn := func() error {
		once.Do(func() {
			l.mu.Lock()
			process.closed++
			l.mu.Unlock()
			_ = conn.Close()
			stop()
			<-done
		})
		return nil
	}
	l.t.Cleanup(func() { _ = closeFn() })
	return conn, closeFn, nil
}

// environments returns the delivered environment of every launch in order.
func (l *credentialLauncher) environments() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	envs := make([][]string, 0, len(l.processes))
	for _, process := range l.processes {
		envs = append(envs, process.env)
	}
	return envs
}

// running returns the environments of the launches that were not closed.
func (l *credentialLauncher) running() [][]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var envs [][]string
	for _, process := range l.processes {
		if process.closed == 0 {
			envs = append(envs, process.env)
		}
	}
	return envs
}

// credentialAPI serves one ESC environment per stack.
func credentialAPI(t *testing.T) *esctest.Server {
	t.Helper()
	stack := func(etag string, variables map[string]esctest.Variable) *esctest.Environment {
		return &esctest.Environment{ETag: etag, Variables: variables}
	}
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/a": stack("a1", map[string]esctest.Variable{credentialVariable: {Value: secretA}}),
		"acme/billing/b": stack("b1", map[string]esctest.Variable{credentialVariable: {Value: secretB}}),
		"acme/billing/none": stack("n1", map[string]esctest.Variable{
			"CIVO_TOKEN": {Value: "civo-secret"},
		}),
		"acme/billing/a-eu": stack("e1", map[string]esctest.Variable{
			credentialVariable: {Value: secretA}, "RECORDER_REGION": {Value: "eu"},
		}),
		"acme/billing/b-us": stack("u1", map[string]esctest.Variable{
			credentialVariable: {Value: secretB}, "RECORDER_REGION": {Value: "us"},
		}),
		"acme/billing/multiline": stack("m1", map[string]esctest.Variable{
			credentialVariable: {Value: secretMultiline},
		}),
		"acme/billing/empty": stack("z1", map[string]esctest.Variable{credentialVariable: {Value: ""}}),
	})
	t.Cleanup(api.Close)
	return api
}

func declaredToken() map[string]string {
	return map[string]string{credentialName: credentialVariable}
}

// stackContext returns a stack's context: its ESC environment and the
// per-plugin options every stack of a dashboard shares.
func stackContext(t *testing.T, api *esctest.Server, env string, options esc.PluginOptions) context.Context {
	t.Helper()
	list := []string{"acme/billing/" + env}
	return esc.WithSource(poolContext(), esc.NewSource(esc.Options{
		Global:    &list,
		PerPlugin: map[string]esc.PluginOptions{poolPlugin: options},
		Getenv: func(key string) string {
			switch key {
			case "PULUMI_ACCESS_TOKEN":
				return "tok"
			case "PULUMI_BACKEND_URL":
				return api.URL
			default:
				return ""
			}
		},
		Budget: time.Second,
	}))
}

func declaredContext(t *testing.T, api *esctest.Server, env string) context.Context {
	t.Helper()
	return stackContext(t, api, env, esc.PluginOptions{Credentials: declaredToken()})
}

func acquire(
	ctx context.Context, t *testing.T, pool *pluginhost.Pool, launcher pluginhost.Launcher,
) *pluginhost.Client {
	t.Helper()
	client, err := pool.Acquire(ctx, launcher, "/plugins/recorder")
	require.NoError(t, err)
	return client
}

// carried returns the credential the plugin read from a call by client.
func carried(t *testing.T, client *pluginhost.Client) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.API.Supports(ctx, &pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{}})
	require.NoError(t, err)
	return resp.GetReason()
}

func tokenEnv(value string) []string { return []string{credentialVariable + "=" + value} }

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_StacksShareOneProcess(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	before := os.Environ()

	a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	b := acquire(declaredContext(t, api, "b"), t, pool, launcher)

	assert.Equal(t, [][]string{tokenEnv(secretA), {}}, normalize(launcher.environments()),
		"the first launch learns that the plugin reads credentials per call, then restarts without them")
	assert.Equal(t, [][]string{{}}, normalize(launcher.running()), "one process, holding neither credential")
	assert.Equal(t, secretA, carried(t, a))
	assert.Equal(t, secretB, carried(t, b))
	assert.Equal(t, secretA, carried(t, a), "a call after the other stack's is unchanged")
	assert.Equal(t, before, os.Environ())
}

// normalize makes a nil environment compare equal to an empty one.
func normalize(envs [][]string) [][]string {
	out := make([][]string, 0, len(envs))
	for _, env := range envs {
		out = append(out, append([]string{}, env...))
	}
	return out
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_ConcurrentCallsCarryTheirOwn(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	b := acquire(declaredContext(t, api, "b"), t, pool, launcher)
	require.Len(t, launcher.running(), 1)

	var wg sync.WaitGroup
	const calls = 50
	seen := make([][2]string, calls)
	for i := range calls {
		wg.Go(func() { seen[i][0] = reason(a) })
		wg.Go(func() { seen[i][1] = reason(b) })
	}
	wg.Wait()
	for _, pair := range seen {
		assert.Equal(t, [2]string{secretA, secretB}, pair)
	}
}

func reason(client *pluginhost.Client) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := client.API.Supports(ctx, &pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{}})
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return resp.GetReason()
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_ConcurrentFirstOpensShare(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	contexts := []context.Context{declaredContext(t, api, "a"), declaredContext(t, api, "b")}
	clients := make([]*pluginhost.Client, len(contexts))
	errs := make([]error, len(contexts))
	var wg sync.WaitGroup
	for i, ctx := range contexts {
		wg.Go(func() { clients[i], errs[i] = pool.Acquire(ctx, launcher, "/plugins/recorder") })
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	assert.Equal(t, [][]string{{}}, normalize(launcher.running()))
	assert.Equal(t, secretA, carried(t, clients[0]))
	assert.Equal(t, secretB, carried(t, clients[1]))
}

// Usage, allocation, scoring, and carbon clients are built from Conn, not API.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_ServicesBuiltFromConnCarryThem(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	client := acquire(declaredContext(t, api, "a"), t, pool, launcher)

	resp, err := pbc.NewCostSourceServiceClient(client.Conn).
		Supports(t.Context(), &pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{}})
	require.NoError(t, err)
	assert.Equal(t, secretA, resp.GetReason())
}

// A stack without the declared variable uses the same process in either order,
// and its calls carry nothing.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_StackWithoutCredentials(t *testing.T) {
	for _, order := range [][2]string{{"a", "none"}, {"none", "a"}} {
		t.Run(order[0]+" first", func(t *testing.T) {
			isolatePoolConfig(t)
			api := credentialAPI(t)
			launcher := &credentialLauncher{t: t}
			pool := newTestPool(t)
			want := map[string]string{"a": secretA, "none": noCredential}
			first := acquire(declaredContext(t, api, order[0]), t, pool, launcher)
			second := acquire(declaredContext(t, api, order[1]), t, pool, launcher)

			assert.Equal(t, [][]string{{}}, normalize(launcher.running()))
			assert.Equal(t, want[order[0]], carried(t, first))
			assert.Equal(t, want[order[1]], carried(t, second))
		})
	}
}

// A shell value is the plugin's ambient credential for every stack. It stays
// in the inherited environment and is never attached to a call.
// Not parallel: it sets the process environment.
func TestPoolCredentials_ShellValueIsNotAttached(t *testing.T) {
	isolatePoolConfig(t)
	t.Setenv(credentialVariable, "ambient-value")
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)

	a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	none := acquire(declaredContext(t, api, "none"), t, pool, launcher)

	require.Len(t, launcher.running(), 1)
	assert.Equal(t, secretA, carried(t, a))
	assert.Equal(t, noCredential, carried(t, none))
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_IsolatedWithoutBothOptIns(t *testing.T) {
	tests := []struct {
		name    string
		specs   []launchSpec
		options esc.PluginOptions
	}{
		{
			name:    "plugin does not advertise support",
			specs:   []launchSpec{{optOut: true}},
			options: esc.PluginOptions{Credentials: declaredToken()},
		},
		{
			name:    "no declared map",
			options: esc.PluginOptions{Variables: []string{credentialVariable}},
		},
		{
			name:    "plugin information is unavailable",
			specs:   []launchSpec{{infoFail: true}},
			options: esc.PluginOptions{Credentials: declaredToken()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolatePoolConfig(t)
			api := credentialAPI(t)
			launcher := &credentialLauncher{t: t, specs: tt.specs}
			pool := newTestPool(t)
			a := acquire(stackContext(t, api, "a", tt.options), t, pool, launcher)
			b := acquire(stackContext(t, api, "b", tt.options), t, pool, launcher)

			assert.Equal(t, [][]string{tokenEnv(secretA), tokenEnv(secretB)}, launcher.environments())
			assert.Len(t, launcher.running(), 2)
			assert.Equal(t, noCredential, carried(t, a))
			assert.Equal(t, noCredential, carried(t, b))
		})
	}
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_NoPoolKeepsTheEnvironment(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}

	client, err := pluginhost.NewClient(declaredContext(t, api, "a"), launcher, "/plugins/recorder")
	require.NoError(t, err)

	assert.Equal(t, [][]string{tokenEnv(secretA)}, launcher.environments())
	assert.Equal(t, noCredential, carried(t, client))
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_OtherDifferencesStaySeparate(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	options := esc.PluginOptions{Credentials: declaredToken(), Variables: []string{"RECORDER_REGION"}}

	a := acquire(stackContext(t, api, "a-eu", options), t, pool, launcher)
	b := acquire(stackContext(t, api, "b-us", options), t, pool, launcher)

	assert.Equal(t, [][]string{{"RECORDER_REGION=eu"}, {"RECORDER_REGION=us"}}, launcher.running())
	assert.Equal(t, secretA, carried(t, a))
	assert.Equal(t, secretB, carried(t, b))
}

// A value the contract cannot carry keeps that stack in its own process.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_UnusableSetIsIsolated(t *testing.T) {
	for env, value := range map[string]string{"multiline": secretMultiline, "empty": ""} {
		t.Run(env, func(t *testing.T) {
			isolatePoolConfig(t)
			api := credentialAPI(t)
			launcher := &credentialLauncher{t: t}
			pool := newTestPool(t)
			var logs lockedBuffer
			ctx := logging.NewLogger(logging.Config{Level: "debug", Format: "json"}).
				Output(&logs).WithContext(declaredContext(t, api, env))

			a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
			odd := acquire(ctx, t, pool, launcher)

			assert.ElementsMatch(t, [][]string{{}, tokenEnv(value)}, normalize(launcher.running()))
			assert.Equal(t, secretA, carried(t, a))
			assert.Equal(t, noCredential, carried(t, odd))
			assert.Contains(t, logs.String(), "per-request credentials are not usable")
			assert.NotContains(t, logs.String(), "test-secret")
		})
	}
}

// The process that launched without credentials must not serve a stack when it
// turns out not to read them from calls.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_SupportLostIsReplacedByAnIsolatedProcess(t *testing.T) {
	lost := map[string]launchSpec{"opted out": {optOut: true}, "information unavailable": {infoFail: true}}
	for name, spec := range lost {
		t.Run("after the first restart, "+name, func(t *testing.T) {
			isolatePoolConfig(t)
			api := credentialAPI(t)
			launcher := &credentialLauncher{t: t, specs: []launchSpec{{}, spec}}
			pool := newTestPool(t)

			a := acquire(declaredContext(t, api, "a"), t, pool, launcher)

			assert.Equal(t, [][]string{tokenEnv(secretA)}, launcher.running())
			assert.Equal(t, noCredential, carried(t, a))
		})
		t.Run("after a predicted launch, "+name, func(t *testing.T) {
			isolatePoolConfig(t)
			api := credentialAPI(t)
			launcher := &credentialLauncher{t: t, specs: []launchSpec{{}, {}, spec}}
			pool := newTestPool(t)
			first := acquire(declaredContext(t, api, "a"), t, pool, launcher)
			require.NoError(t, first.Close())

			b := acquire(declaredContext(t, api, "b"), t, pool, launcher)

			assert.Equal(t, [][]string{tokenEnv(secretB)}, launcher.running())
			assert.Equal(t, noCredential, carried(t, b))
		})
	}
}

// A process of a plugin that reported no information holds no credentials for
// a stack that has none. Another stack's credentials cannot be attached to it.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_UnsupportingProcessIsNotReusedForWithheldCredentials(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t, specs: []launchSpec{{infoFail: true}, {}}}
	pool := newTestPool(t)

	none := acquire(declaredContext(t, api, "none"), t, pool, launcher)
	a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	b := acquire(declaredContext(t, api, "b"), t, pool, launcher)

	assert.Equal(t, noCredential, carried(t, none))
	assert.Equal(t, secretA, carried(t, a))
	assert.Equal(t, secretB, carried(t, b))
	assert.Len(t, launcher.running(), 2, "the unsupporting process and the shared one")
}

// Removing the map while the dashboard runs returns later opens to isolation.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_RemovedMapIsIsolated(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	both := esc.PluginOptions{Credentials: declaredToken(), Variables: []string{credentialVariable}}
	variables := esc.PluginOptions{Variables: []string{credentialVariable}}

	before := acquire(stackContext(t, api, "a", both), t, pool, launcher)
	after := acquire(stackContext(t, api, "a", variables), t, pool, launcher)

	assert.ElementsMatch(t, [][]string{{}, tokenEnv(secretA)}, normalize(launcher.running()))
	assert.Equal(t, secretA, carried(t, before))
	assert.Equal(t, noCredential, carried(t, after))
}

// Two opens with the same ESC offer and different maps resolve the same launch
// environment, but the one without a map needs its values in the environment.
// It must not join a launch that ends without them.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_MapChangeDuringFirstLaunchIsNotShared(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t, gate: make(chan struct{})}
	pool := newTestPool(t)
	var open sync.Once
	release := func() { open.Do(func() { close(launcher.gate) }) }
	t.Cleanup(release)
	options := []esc.PluginOptions{
		{Credentials: declaredToken(), Variables: []string{credentialVariable}},
		{Variables: []string{credentialVariable}},
	}
	clients := make([]*pluginhost.Client, len(options))
	errs := make([]error, len(options))
	var wg sync.WaitGroup
	for i, option := range options {
		ctx := stackContext(t, api, "a", option)
		wg.Go(func() { clients[i], errs[i] = pool.Acquire(ctx, launcher, "/plugins/recorder") })
		require.Eventually(t, func() bool {
			return pluginhost.PoolHoldsForTest(pool)+pluginhost.PoolLearnWaitersForTest(pool) == i+1
		}, 5*time.Second, time.Millisecond, "the open never reached the pool")
	}
	release()
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	assert.ElementsMatch(t, [][]string{{}, tokenEnv(secretA)}, normalize(launcher.running()))
	assert.Equal(t, secretA, carried(t, clients[0]))
	assert.Equal(t, noCredential, carried(t, clients[1]))
}

// A reload after ESC changed takes the new value without another process, and
// the client that is still open keeps the value it resolved.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_ReloadTakesTheNewValue(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	before := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	api.Update("acme", "billing", "a", func(env *esctest.Environment) {
		env.ETag = "a2"
		env.Variables = map[string]esctest.Variable{credentialVariable: {Value: "test-secret-rotated"}}
	})

	after := acquire(declaredContext(t, api, "a"), t, pool, launcher)

	assert.Len(t, launcher.running(), 1)
	assert.Equal(t, secretA, carried(t, before))
	assert.Equal(t, "test-secret-rotated", carried(t, after))
	assert.NotEqual(t, before.DeliveredDigest, after.DeliveredDigest)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_ReleasingOneStackKeepsTheOther(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	a := acquire(declaredContext(t, api, "a"), t, pool, launcher)
	b := acquire(declaredContext(t, api, "b"), t, pool, launcher)

	require.NoError(t, a.Close())
	assert.Equal(t, secretB, carried(t, b))
	require.NoError(t, b.Close())
	assert.Empty(t, launcher.running())
}

// Cost cache keys hash the digest, so it must tell stacks apart on a shared
// process.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_DigestTellsStacksApart(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	digest := func(env string) string {
		return acquire(declaredContext(t, api, env), t, pool, launcher).DeliveredDigest
	}

	a, reopened := digest("a"), digest("a")
	assert.NotEmpty(t, a)
	assert.Equal(t, a, reopened)
	assert.NotEqual(t, a, digest("b"))
	assert.NotEqual(t, a, digest("none"))

	isolated, err := pluginhost.NewClient(declaredContext(t, api, "none"), launcher, "/plugins/recorder")
	require.NoError(t, err)
	assert.Equal(t, isolated.DeliveredDigest, digest("none"), "calls that carry nothing keep their keys")
}

// Remapping a name to another variable the plugin already receives changes the
// account a call uses, and nothing else. Cached costs must not survive it.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_DigestFollowsTheMap(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t}
	pool := newTestPool(t)
	mapped := func(variable string) *pluginhost.Client {
		return acquire(stackContext(t, api, "a-eu", esc.PluginOptions{
			Credentials: map[string]string{credentialName: variable},
			Variables:   []string{credentialVariable, "RECORDER_REGION"},
		}), t, pool, launcher)
	}
	before, after := mapped(credentialVariable), mapped("RECORDER_REGION")

	assert.Equal(t, secretA, carried(t, before))
	assert.Equal(t, "eu", carried(t, after))
	assert.NotEqual(t, before.DeliveredDigest, after.DeliveredDigest)
}

// lockedBuffer collects log lines written from several goroutines.
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

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPoolCredentials_EchoedSecretIsMasked(t *testing.T) {
	isolatePoolConfig(t)
	api := credentialAPI(t)
	launcher := &credentialLauncher{t: t, specs: []launchSpec{{echo: true}}}
	pool := newTestPool(t)
	var logs lockedBuffer
	logger := logging.NewLogger(logging.Config{Level: "debug", Format: "json"}).Output(&logs)
	client := acquire(logger.WithContext(declaredContext(t, api, "a")), t, pool, launcher)

	_, err := client.API.Supports(logger.WithContext(t.Context()),
		&pbc.SupportsRequest{Resource: &pbc.ResourceDescriptor{}})

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Contains(t, err.Error(), "backend refused")
	assert.NotContains(t, err.Error(), secretA)
	assert.Contains(t, logs.String(), "backend refused")
	assert.NotContains(t, logs.String(), secretA)
	assert.NotContains(t, logs.String(), pluginsdk.CredentialMetadataPrefix)
}
