package registry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

const fakeBufSize = 1024 * 1024

type namedCostSource struct {
	pbc.UnimplementedCostSourceServiceServer

	name      string
	calls     *atomic.Int32
	infoCalls *atomic.Int32
}

func (s *namedCostSource) Name(context.Context, *pbc.NameRequest) (*pbc.NameResponse, error) {
	s.calls.Add(1)
	return &pbc.NameResponse{Name: s.name}, nil
}

// GetPluginInfo is called by NewClient only after Name has returned, and its
// failure does not drop the client, so a call here marks a connected client.
func (s *namedCostSource) GetPluginInfo(
	context.Context,
	*pbc.GetPluginInfoRequest,
) (*pbc.GetPluginInfoResponse, error) {
	s.infoCalls.Add(1)
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// fakePluginSpec controls how fakeLauncher starts one plugin.
type fakePluginSpec struct {
	delay     time.Duration
	fail      bool
	waitOnCtx bool
}

// fakeLauncher serves each plugin from an in-process gRPC server, so Open can
// be exercised without executing any binary.
type fakeLauncher struct {
	specs       map[string]fakePluginSpec
	defaultSpec fakePluginSpec
	listeners   map[string]*bufconn.Listener

	mu        sync.Mutex
	started   []string
	inFlight  atomic.Int32
	maxFlight atomic.Int32
	closed    atomic.Int32
	nameCalls atomic.Int32
	infoCalls atomic.Int32
}

func newFakeLauncher(t testing.TB, names []string, specs map[string]fakePluginSpec) *fakeLauncher {
	t.Helper()
	l := &fakeLauncher{specs: specs, listeners: make(map[string]*bufconn.Listener, len(names))}
	for _, name := range names {
		lis := bufconn.Listen(fakeBufSize)
		srv := grpc.NewServer()
		pbc.RegisterCostSourceServiceServer(
			srv,
			&namedCostSource{name: name, calls: &l.nameCalls, infoCalls: &l.infoCalls},
		)
		go func() { _ = srv.Serve(lis) }()
		t.Cleanup(func() {
			srv.Stop()
			_ = lis.Close()
		})
		l.listeners[name] = lis
	}
	return l
}

func (l *fakeLauncher) Start(
	ctx context.Context,
	path string,
	_ ...string,
) (*grpc.ClientConn, func() error, error) {
	name := strings.TrimSuffix(filepath.Base(path), ".exe")

	current := l.inFlight.Add(1)
	defer l.inFlight.Add(-1)
	for {
		seen := l.maxFlight.Load()
		if current <= seen || l.maxFlight.CompareAndSwap(seen, current) {
			break
		}
	}

	l.mu.Lock()
	l.started = append(l.started, name)
	l.mu.Unlock()

	spec, ok := l.specs[name]
	if !ok {
		spec = l.defaultSpec
	}
	if spec.waitOnCtx {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	select {
	case <-time.After(spec.delay):
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	if spec.fail {
		return nil, nil, fmt.Errorf("launching %s: %w", name, errors.New("fake launch failure"))
	}

	lis := l.listeners[name]
	conn, err := grpc.NewClient(
		"passthrough:///"+name,
		grpc.WithContextDialer(func(dialCtx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(dialCtx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	return conn, func() error {
		l.closed.Add(1)
		return conn.Close()
	}, nil
}

func (l *fakeLauncher) startedNames() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.started...)
}

func pluginNames(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("plugin-%02d", i)
	}
	return names
}

func createPluginsDir(t testing.TB, names []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		require.NoError(t, createPluginVersion(dir, name, "v1.0.0"))
	}
	return dir
}

func clientNames(clients []*pluginhost.Client) []string {
	names := make([]string, len(clients))
	for i, c := range clients {
		names[i] = c.Name
	}
	return names
}

func TestRegistry_Open_Concurrent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		plugins   int
		specs     map[string]fakePluginSpec
		wantNames []string
	}{
		{
			name:      "opens every plugin",
			plugins:   3,
			wantNames: []string{"plugin-00", "plugin-01", "plugin-02"},
		},
		{
			name:    "keeps discovery order when later plugins finish first",
			plugins: 4,
			specs: map[string]fakePluginSpec{
				"plugin-00": {delay: 120 * time.Millisecond},
				"plugin-01": {delay: 80 * time.Millisecond},
				"plugin-02": {delay: 40 * time.Millisecond},
			},
			wantNames: []string{"plugin-00", "plugin-01", "plugin-02", "plugin-03"},
		},
		{
			name:    "a failing plugin does not block the others",
			plugins: 3,
			specs: map[string]fakePluginSpec{
				"plugin-01": {fail: true},
			},
			wantNames: []string{"plugin-00", "plugin-02"},
		},
		{
			name:    "all plugins failing returns no clients",
			plugins: 2,
			specs: map[string]fakePluginSpec{
				"plugin-00": {fail: true},
				"plugin-01": {fail: true},
			},
			wantNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			names := pluginNames(tt.plugins)
			launcher := newFakeLauncher(t, names, tt.specs)
			reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

			clients, cleanup, err := reg.Open(context.Background(), "")
			require.NoError(t, err)
			require.NotNil(t, cleanup)

			assert.Equal(t, tt.wantNames, clientNames(clients))
			assert.ElementsMatch(t, names, launcher.startedNames())

			cleanup()
			assert.Equal(t, int32(len(tt.wantNames)), launcher.closed.Load(),
				"cleanup closes exactly the connected clients")
		})
	}
}

func TestRegistry_Open_LaunchesInParallel(t *testing.T) {
	t.Parallel()

	names := pluginNames(4)
	launcher := newFakeLauncher(t, names, nil)
	launcher.defaultSpec = fakePluginSpec{delay: 100 * time.Millisecond}
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

	clients, cleanup, err := reg.Open(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(cleanup)

	assert.Len(t, clients, len(names))
	assert.GreaterOrEqual(t, launcher.maxFlight.Load(), int32(2),
		"plugins should launch concurrently")
}

func TestRegistry_Open_BoundsConcurrency(t *testing.T) {
	t.Parallel()

	names := pluginNames(maxConcurrentPluginOpens * 2)
	launcher := newFakeLauncher(t, names, nil)
	launcher.defaultSpec = fakePluginSpec{delay: 20 * time.Millisecond}
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

	clients, cleanup, err := reg.Open(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(cleanup)

	assert.Equal(t, names, clientNames(clients))
	assert.LessOrEqual(t, launcher.maxFlight.Load(), int32(maxConcurrentPluginOpens))
}

func TestRegistry_Open_FilterOpensOnlyNamedPlugin(t *testing.T) {
	t.Parallel()

	names := pluginNames(3)
	launcher := newFakeLauncher(t, names, nil)
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

	clients, cleanup, err := reg.Open(context.Background(), "plugin-01")
	require.NoError(t, err)
	t.Cleanup(cleanup)

	assert.Equal(t, []string{"plugin-01"}, clientNames(clients))
	assert.Equal(t, []string{"plugin-01"}, launcher.startedNames())
}

func TestRegistry_Open_ContextCancelled(t *testing.T) {
	t.Parallel()

	names := pluginNames(3)
	launcher := newFakeLauncher(t, names, map[string]fakePluginSpec{
		"plugin-01": {waitOnCtx: true},
	})
	reg := &Registry{root: createPluginsDir(t, names), launcher: launcher}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for launcher.infoCalls.Load() < 2 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
	}()

	clients, cleanup, err := reg.Open(ctx, "")
	require.NoError(t, err)

	assert.Zero(t, launcher.inFlight.Load(), "Open must not return while a launch is still running")
	assert.Equal(t, []string{"plugin-00", "plugin-02"}, clientNames(clients))

	cleanup()
	assert.Equal(t, int32(2), launcher.closed.Load())
}

func BenchmarkOpen(b *testing.B) {
	const plugins = 4
	const launchDelay = 20 * time.Millisecond

	names := pluginNames(plugins)
	launcher := newFakeLauncher(b, names, nil)
	launcher.defaultSpec = fakePluginSpec{delay: launchDelay}
	reg := &Registry{root: createPluginsDir(b, names), launcher: launcher}
	ctx := context.Background()

	b.Run("sequential", func(b *testing.B) {
		for b.Loop() {
			discovered, _, err := reg.ListLatestPlugins()
			require.NoError(b, err)
			for _, plugin := range discovered {
				client, clientErr := pluginhost.NewClient(ctx, launcher, plugin.Path)
				require.NoError(b, clientErr)
				_ = client.Close()
			}
		}
	})

	b.Run("concurrent", func(b *testing.B) {
		for b.Loop() {
			clients, cleanup, err := reg.Open(ctx, "")
			require.NoError(b, err)
			require.Len(b, clients, plugins)
			cleanup()
		}
	})
}
