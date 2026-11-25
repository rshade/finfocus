package pluginhost_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// credentialProcessEnv makes this test binary serve a plugin instead of tests.
const credentialProcessEnv = "FINFOCUS_TEST_CREDENTIAL_PLUGIN"

func TestMain(m *testing.M) {
	mode := os.Getenv(credentialProcessEnv)
	if mode == "" {
		os.Exit(m.Run())
	}
	if err := serveCredentialProcess(mode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// reportingPlugin answers Supports with the call's credential, its own
// environment's value, and its process id.
type reportingPlugin struct{ *pluginsdk.BasePlugin }

func (reportingPlugin) Supports(ctx context.Context, _ *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	creds, err := pluginsdk.ExtractCredentials(ctx)
	if err != nil {
		return nil, err
	}
	call, _ := creds.Get(credentialName)
	return &pbc.SupportsResponse{Supported: true, Reason: strings.Join(
		[]string{call, os.Getenv(credentialVariable), strconv.Itoa(os.Getpid())}, "|")}, nil
}

type optedInReportingPlugin struct{ reportingPlugin }

func (optedInReportingPlugin) ConsumesPerRequestCredentials() {}

func serveCredentialProcess(mode string) error {
	port := 0
	for i, arg := range os.Args {
		value, inline := strings.CutPrefix(arg, "--port=")
		if arg == "--port" && i+1 < len(os.Args) {
			value, inline = os.Args[i+1], true
		}
		if inline {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			port = parsed
		}
	}
	plugin := reportingPlugin{pluginsdk.NewBasePlugin(poolPlugin)}
	var served pluginsdk.Plugin = plugin
	if mode == "unannounced" {
		null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		os.Stdout = null //nolint:reassign // the SDK announces the port on os.Stdout
	}
	if mode != "opted-out" {
		served = optedInReportingPlugin{plugin}
	}
	quiet := zerolog.Nop()
	return pluginsdk.Serve(context.Background(), pluginsdk.ServeConfig{
		Plugin: served, Port: port, Logger: &quiet,
		PluginInfo: pluginsdk.NewPluginInfo(poolPlugin, "v1.0.0"),
	})
}

type processReport struct{ call, env, pid string }

func report(t *testing.T, client *pluginhost.Client) processReport {
	t.Helper()
	parts := strings.Split(carried(t, client), "|")
	require.Len(t, parts, 3)
	return processReport{call: parts[0], env: parts[1], pid: parts[2]}
}

// A process that will receive credentials must prove that it owns the port the
// host connects to, by announcing it on its own output. A plugin whose output
// does not say so keeps its credentials in its environment.
// Not parallel: it sets the process environment.
func TestPoolCredentials_UnannouncedPortIsIsolated(t *testing.T) {
	isolatePoolConfig(t)
	t.Setenv(credentialProcessEnv, "unannounced")
	binary, err := os.Executable()
	require.NoError(t, err)
	api := credentialAPI(t)
	pool := newTestPool(t)
	launcher := pluginhost.NewProcessLauncher()
	const wait = 3 * time.Second
	pluginhost.SetBindTimeoutForTest(launcher, wait)
	pids := map[string]bool{}
	started := time.Now()
	// The stack without credentials comes first: its process would need no
	// restart to be shared, and must prove itself all the same.
	for _, stack := range []struct{ env, value string }{{"none", ""}, {"a", secretA}, {"b", secretB}} {
		client, acquireErr := pool.Acquire(declaredContext(t, api, stack.env), launcher, binary)
		require.NoError(t, acquireErr)
		got := report(t, client)
		assert.Equal(t, processReport{env: stack.value, pid: got.pid}, got, stack.env)
		pids[got.pid] = true
	}
	assert.Len(t, pids, 3)
	assert.Less(t, time.Since(started), 2*wait, "only the first open waits for an announcement")
}

// The whole path through real plugin processes: what the plugin sees in its
// own environment and on each call.
// Not parallel: it sets the process environment.
func TestPoolCredentials_RealProcesses(t *testing.T) {
	tests := []struct {
		mode         string
		wantA, wantB processReport
		shared       bool
	}{
		{mode: "opted-in", wantA: processReport{call: secretA}, wantB: processReport{call: secretB}, shared: true},
		{mode: "opted-out", wantA: processReport{env: secretA}, wantB: processReport{env: secretB}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			isolatePoolConfig(t)
			t.Setenv(credentialProcessEnv, tt.mode)
			binary, err := os.Executable()
			require.NoError(t, err)
			api := credentialAPI(t)
			pool := newTestPool(t)
			launcher := pluginhost.NewProcessLauncher()
			open := func(env string) *pluginhost.Client {
				client, openErr := pool.Acquire(declaredContext(t, api, env), launcher, binary)
				require.NoError(t, openErr)
				return client
			}

			a, b := report(t, open("a")), report(t, open("b"))

			assert.Equal(t, tt.shared, a.pid == b.pid)
			a.pid, b.pid = "", ""
			assert.Equal(t, tt.wantA, a)
			assert.Equal(t, tt.wantB, b)
		})
	}
}
