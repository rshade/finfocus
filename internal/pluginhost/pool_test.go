package pluginhost_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

const poolPlugin = "recorder"

// fakeProcess is one launch: its own gRPC server stands in for the process.
type fakeProcess struct {
	ctx    context.Context
	env    []string
	server *grpc.Server
	closed int
}

// processLauncher starts a separate in-memory server for every Start, so
// tests can count launches, read each launch's environment, and kill one.
type processLauncher struct {
	info  *pbc.GetPluginInfoResponse
	gate  chan struct{}
	fail  error
	names atomic.Int32

	mu        sync.Mutex
	processes []*fakeProcess
}

func (l *processLauncher) Start(
	ctx context.Context, _ string, _ ...string,
) (*grpc.ClientConn, func() error, error) {
	if l.gate != nil {
		<-l.gate
	}
	l.mu.Lock()
	fail := l.fail
	l.fail = nil
	l.mu.Unlock()
	if fail != nil {
		return nil, nil, fail
	}
	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		if strings.HasSuffix(info.FullMethod, "/Name") {
			l.names.Add(1)
		}
		return handler(ctx, req)
	}))
	pbc.RegisterCostSourceServiceServer(server, &mockCostSourceServer{name: poolPlugin, pluginInfo: l.info})
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(dialCtx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		server.Stop()
		return nil, nil, err
	}
	delivered, _ := pluginhost.DeliveredEnvFromContext(ctx)
	process := &fakeProcess{ctx: ctx, env: append([]string(nil), delivered.Pairs...), server: server}
	l.mu.Lock()
	l.processes = append(l.processes, process)
	l.mu.Unlock()
	return conn, func() error {
		l.mu.Lock()
		process.closed++
		l.mu.Unlock()
		server.Stop()
		return conn.Close()
	}, nil
}

func (l *processLauncher) starts() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.processes)
}

func (l *processLauncher) process(t *testing.T, i int) *fakeProcess {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	require.Greater(t, len(l.processes), i)
	return l.processes[i]
}

func (l *processLauncher) closes(i int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.processes[i].closed
}

func poolPluginInfo(settings ...*pbc.PluginSetting) *pbc.GetPluginInfoResponse {
	return &pbc.GetPluginInfoResponse{
		Name: poolPlugin, Version: "1.0.0", SpecVersion: pluginsdk.SpecVersion, Settings: settings,
	}
}

func isolatePoolConfig(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_PROJECT_DIR", t.TempDir())
}

func poolContext() context.Context {
	return pluginhost.WithInstallName(context.Background(), poolPlugin)
}

func waitForHolds(t *testing.T, pool *pluginhost.Pool, holds int) {
	t.Helper()
	require.Eventually(t, func() bool { return pluginhost.PoolHoldsForTest(pool) == holds },
		5*time.Second, time.Millisecond, "requesters never joined the launch")
}

func newTestPool(t *testing.T) *pluginhost.Pool {
	t.Helper()
	pool := pluginhost.NewPool(context.Background())
	t.Cleanup(pool.Close)
	return pool
}

func answers(t *testing.T, client *pluginhost.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := client.API.Name(ctx, &proto.Empty{})
	require.NoError(t, err)
	assert.Equal(t, poolPlugin, resp.GetName())
}

// stackESC returns a stack's ESC source over env, which offers value for the
// advertised variable FINFOCUS_RECORDER_MOCK_RESPONSE.
func stackESC(t *testing.T, api *esctest.Server, env string, variables ...string) *esc.Source {
	t.Helper()
	list := []string{env}
	opts := esc.Options{
		Global: &list,
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
	}
	if len(variables) > 0 {
		opts.PerPlugin = map[string]esc.PluginOptions{poolPlugin: {Variables: variables}}
	}
	return esc.NewSource(opts)
}

func twoStackESC(t *testing.T) *esctest.Server {
	t.Helper()
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/a": {ETag: "a1", Variables: map[string]esctest.Variable{
			"FINFOCUS_RECORDER_MOCK_RESPONSE": {Value: "from-a"},
		}},
		"acme/billing/b": {ETag: "b1", Variables: map[string]esctest.Variable{
			"FINFOCUS_RECORDER_MOCK_RESPONSE": {Value: "from-b"},
		}},
		"acme/billing/c": {ETag: "c1", Variables: map[string]esctest.Variable{
			"FINFOCUS_RECORDER_MOCK_RESPONSE": {Value: "from-a"},
		}},
	})
	t.Cleanup(api.Close)
	return api
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_SameEnvironmentLaunchesOnce(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)

	first, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	second, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)

	assert.Equal(t, 1, launcher.starts())
	answers(t, first)
	answers(t, second)
	require.NotNil(t, first.Metadata)
	assert.NotSame(t, first.Metadata, second.Metadata, "each holder needs its own metadata")
	assert.NotSame(t, first, second)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_DifferentBinaryLaunchesSeparately(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)

	_, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder/v1/recorder")
	require.NoError(t, err)
	_, err = pool.Acquire(poolContext(), launcher, "/plugins/recorder/v2/recorder")
	require.NoError(t, err)

	assert.Equal(t, 2, launcher.starts())
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_ConcurrentAcquiresLaunchOnce(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo(), gate: make(chan struct{})}
	pool := newTestPool(t)

	const holders = 8
	clients := make([]*pluginhost.Client, holders)
	errs := make([]error, holders)
	var wg sync.WaitGroup
	for i := range holders {
		wg.Go(func() {
			clients[i], errs[i] = pool.Acquire(poolContext(), launcher, "/plugins/recorder")
		})
	}
	waitForHolds(t, pool, holders)
	close(launcher.gate)
	wg.Wait()
	assert.Equal(t, int32(1), launcher.names.Load(), "every holder joined the launch; none probed a running process")

	for i := range holders {
		require.NoError(t, errs[i])
		answers(t, clients[i])
	}
	assert.Equal(t, 1, launcher.starts())
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_SharedDigestMatchesUnsharedClient(t *testing.T) {
	isolatePoolConfig(t)
	info := poolPluginInfo(&pbc.PluginSetting{
		Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE", DefaultValue: "from-default",
	})
	launcher := &processLauncher{info: info}
	pool := newTestPool(t)

	shared, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	direct, err := pluginhost.NewClient(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	t.Cleanup(func() { _ = direct.Close() })

	assert.NotEmpty(t, shared.DeliveredDigest)
	assert.Equal(t, direct.DeliveredDigest, shared.DeliveredDigest)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_DifferentLaunchEnvironmentIsNotShared(t *testing.T) {
	isolatePoolConfig(t)
	api := twoStackESC(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)
	stackA := esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/a", "FINFOCUS_RECORDER_MOCK_RESPONSE"))

	_, err := pool.Acquire(stackA, launcher, "/plugins/recorder")
	require.NoError(t, err)
	_, err = pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)

	require.Equal(t, 2, launcher.starts())
	assert.Contains(t, launcher.process(t, 0).env, "FINFOCUS_RECORDER_MOCK_RESPONSE=from-a")
	assert.NotContains(t, launcher.process(t, 1).env, "FINFOCUS_RECORDER_MOCK_RESPONSE=from-a")
}

// An ESC value for an advertised setting is not allowed before the first
// launch, so both stacks resolve the same launch environment. Only the
// restart after GetPluginInfo delivers each stack's value, and sharing by the
// launch environment alone would give stack b stack a's credential.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_DifferentESCOfferForAdvertisedSettingIsNotShared(t *testing.T) {
	isolatePoolConfig(t)
	api := twoStackESC(t)
	launcher := &processLauncher{info: poolPluginInfo(&pbc.PluginSetting{
		Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE",
	})}
	pool := newTestPool(t)
	stackA := esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/a"))
	stackB := esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/b"))

	_, err := pool.Acquire(stackA, launcher, "/plugins/recorder")
	require.NoError(t, err)
	_, err = pool.Acquire(stackB, launcher, "/plugins/recorder")
	require.NoError(t, err)

	require.Equal(t, 4, launcher.starts(), "each stack launches, then restarts for its advertised value")
	assert.Empty(t, launcher.process(t, 0).env)
	assert.Equal(t, []string{"FINFOCUS_RECORDER_MOCK_RESPONSE=from-a"}, launcher.process(t, 1).env)
	assert.Empty(t, launcher.process(t, 2).env)
	assert.Equal(t, []string{"FINFOCUS_RECORDER_MOCK_RESPONSE=from-b"}, launcher.process(t, 3).env)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_SameESCOfferIsShared(t *testing.T) {
	isolatePoolConfig(t)
	api := twoStackESC(t)
	launcher := &processLauncher{info: poolPluginInfo(&pbc.PluginSetting{
		Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE",
	})}
	pool := newTestPool(t)

	_, err := pool.Acquire(esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/a")), launcher, "/p")
	require.NoError(t, err)
	_, err = pool.Acquire(esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/a")), launcher, "/p")
	require.NoError(t, err)

	assert.Equal(t, 2, launcher.starts(), "one launch and its one advertised-settings restart")
}

// The delivered digest, which feeds projected cache keys, hashes an ESC value
// by reference and revision. Stacks that receive the same value from different
// environments therefore keep their own process and their own digest.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_SameValueFromDifferentESCEnvironmentIsNotShared(t *testing.T) {
	isolatePoolConfig(t)
	api := twoStackESC(t)
	launcher := &processLauncher{info: poolPluginInfo(&pbc.PluginSetting{
		Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE",
	})}
	pool := newTestPool(t)

	fromA, err := pool.Acquire(esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/a")), launcher, "/p")
	require.NoError(t, err)
	fromC, err := pool.Acquire(esc.WithSource(poolContext(), stackESC(t, api, "acme/billing/c")), launcher, "/p")
	require.NoError(t, err)

	assert.Equal(t, 4, launcher.starts())
	assert.NotEqual(t, fromA.DeliveredDigest, fromC.DeliveredDigest)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_ReinstalledBinaryAtSamePathLaunchesAgain(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)
	binPath := filepath.Join(t.TempDir(), "finfocus-plugin-recorder")
	require.NoError(t, os.WriteFile(binPath, []byte("v1"), 0o600))

	_, err := pool.Acquire(poolContext(), launcher, binPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(binPath, []byte("v1-reinstalled"), 0o600))
	_, err = pool.Acquire(poolContext(), launcher, binPath)
	require.NoError(t, err)

	assert.Equal(t, 2, launcher.starts(), "a held process of the old file is not handed to the reload")
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_CancelledRequesterLaunchesNothing(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)
	ctx, cancel := context.WithCancel(poolContext())
	cancel()

	_, err := pool.Acquire(ctx, launcher, "/plugins/recorder")
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, launcher.starts())
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_CancellingRequesterDoesNotStopSharedProcess(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)
	stackCtx, cancelStack := context.WithCancel(poolContext())

	first, err := pool.Acquire(stackCtx, launcher, "/plugins/recorder")
	require.NoError(t, err)
	second, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	cancelStack()
	require.NoError(t, first.Close())

	require.NoError(t, launcher.process(t, 0).ctx.Err(), "the launch context belongs to the pool")
	assert.Equal(t, 0, launcher.closes(0))
	answers(t, second)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_LastReleaseClosesOnce(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)

	first, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	second, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)

	require.NoError(t, first.Close())
	require.NoError(t, first.Close())
	assert.Equal(t, 0, launcher.closes(0), "a double close must not release another holder's hold")
	require.NoError(t, second.Close())
	assert.Equal(t, 1, launcher.closes(0))
	require.Error(t, launcher.process(t, 0).ctx.Err(), "closing the entry ends its launch context")

	third, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	assert.Equal(t, 2, launcher.starts(), "a released process is not reused")
	answers(t, third)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_RequesterCancelledDuringLaunchLeavesNoProcess(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo(), gate: make(chan struct{})}
	pool := newTestPool(t)
	ctx, cancel := context.WithCancel(poolContext())

	done := make(chan error, 1)
	go func() {
		_, err := pool.Acquire(ctx, launcher, "/plugins/recorder")
		done <- err
	}()
	waitForHolds(t, pool, 1)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)

	close(launcher.gate)
	require.Eventually(t, func() bool {
		return launcher.starts() == 1 && launcher.closes(0) == 1
	}, 5*time.Second, 10*time.Millisecond)
}

// A launch every requester gave up on must not block the key: a launch that
// never answers would otherwise fail every later request until shutdown.
//
//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_AbandonedLaunchDoesNotBlockNextRequest(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo(), gate: make(chan struct{})}
	pool := newTestPool(t)
	ctx, cancel := context.WithCancel(poolContext())

	abandoned := make(chan error, 1)
	go func() {
		_, err := pool.Acquire(ctx, launcher, "/plugins/recorder")
		abandoned <- err
	}()
	waitForHolds(t, pool, 1)
	cancel()
	require.ErrorIs(t, <-abandoned, context.Canceled)

	type result struct {
		client *pluginhost.Client
		err    error
	}
	next := make(chan result, 1)
	go func() {
		client, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
		next <- result{client, err}
	}()
	waitForHolds(t, pool, 1)
	close(launcher.gate)
	got := <-next
	require.NoError(t, got.err)
	answers(t, got.client)

	require.Eventually(t, func() bool { return launcher.starts() == 2 }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return launcher.closes(0)+launcher.closes(1) == 1
	}, 5*time.Second, 10*time.Millisecond, "only the abandoned launch is closed")
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_FailedLaunchReachesWaitersAndIsRetried(t *testing.T) {
	isolatePoolConfig(t)
	launchErr := errors.New("bind timeout")
	launcher := &processLauncher{info: poolPluginInfo(), gate: make(chan struct{}), fail: launchErr}
	pool := newTestPool(t)

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
			errs <- err
		}()
	}
	waitForHolds(t, pool, 2)
	close(launcher.gate)
	require.ErrorIs(t, <-errs, launchErr)
	require.ErrorIs(t, <-errs, launchErr)

	client, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	answers(t, client)
	assert.Equal(t, 1, launcher.starts())
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_DeadProcessIsReplaced(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := newTestPool(t)

	held, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	launcher.process(t, 0).server.Stop()

	fresh, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	assert.Equal(t, 2, launcher.starts())
	answers(t, fresh)

	require.NoError(t, held.Close())
	assert.Equal(t, 1, launcher.closes(0), "the evicted process closes when its last holder releases")
	assert.Equal(t, 0, launcher.closes(1))
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_CloseStopsHeldProcessesAndRejectsAcquire(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo()}
	pool := pluginhost.NewPool(context.Background())

	held, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.NoError(t, err)
	pool.Close()
	pool.Close()

	assert.Equal(t, 1, launcher.closes(0))
	require.Error(t, launcher.process(t, 0).ctx.Err())
	require.NoError(t, held.Close())
	assert.Equal(t, 1, launcher.closes(0))
	_, err = pool.Acquire(poolContext(), launcher, "/plugins/recorder")
	require.ErrorIs(t, err, pluginhost.ErrPoolClosed)
}

//nolint:paralleltest // isolatePoolConfig sets FINFOCUS_HOME and FINFOCUS_PROJECT_DIR
func TestPool_CloseStopsLaunchInProgress(t *testing.T) {
	isolatePoolConfig(t)
	launcher := &processLauncher{info: poolPluginInfo(), gate: make(chan struct{})}
	pool := pluginhost.NewPool(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := pool.Acquire(poolContext(), launcher, "/plugins/recorder")
		done <- err
	}()
	waitForHolds(t, pool, 1)
	closed := make(chan struct{})
	go func() {
		pool.Close()
		close(closed)
	}()
	require.Eventually(t, func() bool { return pluginhost.PoolClosedForTest(pool) },
		5*time.Second, time.Millisecond)
	close(launcher.gate)
	<-closed

	require.Error(t, <-done)
	require.Equal(t, 1, launcher.starts())
	assert.Equal(t, 1, launcher.closes(0))
}

func TestPoolFromContext(t *testing.T) {
	t.Parallel()

	_, ok := pluginhost.PoolFromContext(context.Background())
	assert.False(t, ok)

	pool := pluginhost.NewPool(context.Background())
	t.Cleanup(pool.Close)
	got, ok := pluginhost.PoolFromContext(pluginhost.WithPool(context.Background(), pool))
	require.True(t, ok)
	assert.Same(t, pool, got)
}
