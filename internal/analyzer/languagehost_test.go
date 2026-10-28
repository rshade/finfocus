package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rshade/finfocus/pkg/version"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("FINFOCUS_LANGHOST_TEST_CHILD"); mode != "" {
		runLanguageHostTestChild(mode)
		return
	}
	os.Exit(runIsolated(m))
}

// runIsolated points FINFOCUS_HOME at a scratch directory. Install sets up the
// policy pack under it, and without it a test run replaces the entries in the
// developer's real ~/.finfocus/analyzer with links to the test binary.
func runIsolated(m *testing.M) int {
	home, err := os.MkdirTemp("", "finfocus-analyzer-test-home-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "isolated home: %v\n", err)
		return 1
	}
	defer os.RemoveAll(home)
	if setErr := os.Setenv("FINFOCUS_HOME", home); setErr != nil {
		fmt.Fprintf(os.Stderr, "isolated home: %v\n", setErr)
		return 1
	}
	return m.Run()
}

func runLanguageHostTestChild(mode string) {
	switch mode {
	case "report":
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, "line-one\n")
		payload := map[string]any{
			"args0":    os.Args[0],
			"args":     os.Args,
			"wd":       wd,
			"sentinel": os.Getenv("FINFOCUS_LANGHOST_SENTINEL"),
			"project":  os.Getenv("FINFOCUS_PROJECT_DIR"),
		}
		if encErr := json.NewEncoder(os.Stdout).Encode(payload); encErr != nil {
			fmt.Fprintf(os.Stderr, "encode: %v\n", encErr)
			os.Exit(1)
		}
	case "stderr":
		fmt.Fprintln(os.Stderr, "err-line")
	case "exit3":
		os.Exit(3)
	case "wait-signal":
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		fmt.Fprintln(os.Stdout, "ready")
		<-ch
		done := os.Getenv("FINFOCUS_LANGHOST_TEST_DONE")
		if done != "" {
			if err := os.WriteFile(done, []byte("ok"), 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "done file: %v\n", err)
				os.Exit(1)
			}
		}
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown child mode %q\n", mode)
		os.Exit(2)
	}
}

func startLanguageHost(t *testing.T, opts LanguageHostOptions) (pulumirpc.LanguageRuntimeClient, *LanguageHost) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	host := NewLanguageHost(opts)
	pulumirpc.RegisterLanguageRuntimeServer(srv, host)
	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			assert.NoError(t, err, "language host serve")
		}
	}()
	t.Cleanup(func() {
		host.Shutdown()
		srv.Stop()
		_ = lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return pulumirpc.NewLanguageRuntimeClient(conn), host
}

func testExecutable(t *testing.T) string {
	t.Helper()
	exe := os.Args[0]
	if filepath.IsAbs(exe) {
		return exe
	}
	abs, err := filepath.Abs(exe)
	require.NoError(t, err)
	return abs
}

func analyzerName() string {
	return policyPackEntryName(policyPackBinaryName, runtime.GOOS)
}

type pluginStreamResult struct {
	stdout []byte
	stderr []byte
	exit   *int32
	err    error
}

type pluginRecver interface {
	Recv() (*pulumirpc.RunPluginResponse, error)
}

func readPluginStream(stream pluginRecver) pluginStreamResult {
	var out pluginStreamResult
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			out.err = err
			return out
		}
		switch msg.GetOutput().(type) {
		case *pulumirpc.RunPluginResponse_Stdout:
			out.stdout = append(out.stdout, msg.GetStdout()...)
		case *pulumirpc.RunPluginResponse_Stderr:
			out.stderr = append(out.stderr, msg.GetStderr()...)
		case *pulumirpc.RunPluginResponse_Exitcode:
			code := msg.GetExitcode()
			out.exit = &code
		}
	}
}

func TestNewLanguageHost_Defaults(t *testing.T) {
	t.Parallel()

	host := NewLanguageHost(LanguageHostOptions{})
	t.Cleanup(host.Shutdown)

	assert.NotEmpty(t, host.opts.Executable)
	assert.Equal(t, analyzerName(), host.opts.AnalyzerName)
	assert.Equal(t, LanguageHostDefaultWaitDelay, host.opts.WaitDelay)
	assert.Equal(t, LanguageHostDefaultWaitDelay+time.Second, host.ShutdownCap())
}

func TestLanguageHost_InfoRPCs(t *testing.T) {
	t.Parallel()

	const exe = "/opt/finfocus/finfocus"
	client, _ := startLanguageHost(t, LanguageHostOptions{Executable: exe, AnalyzerName: "name"})
	ctx := context.Background()

	info, err := client.GetPluginInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, version.GetVersion(), info.GetVersion())

	handshake, err := client.Handshake(ctx, &pulumirpc.LanguageHandshakeRequest{})
	require.NoError(t, err)
	assert.NotNil(t, handshake)

	// The language host still serves GetRequiredPlugins.
	plugins, err := client.GetRequiredPlugins( //nolint:staticcheck // RPC remains part of the host contract
		ctx, &pulumirpc.GetRequiredPluginsRequest{})
	require.NoError(t, err)
	assert.Empty(t, plugins.GetPlugins())

	packages, err := client.GetRequiredPackages(ctx, &pulumirpc.GetRequiredPackagesRequest{})
	require.NoError(t, err)
	assert.Empty(t, packages.GetPackages())

	deps, err := client.GetProgramDependencies(ctx, &pulumirpc.GetProgramDependenciesRequest{})
	require.NoError(t, err)
	assert.Empty(t, deps.GetDependencies())

	install, err := client.InstallDependencies(ctx, &pulumirpc.InstallDependenciesRequest{})
	require.NoError(t, err)
	_, err = install.Recv()
	require.ErrorIs(t, err, io.EOF)

	about, err := client.About(ctx, &pulumirpc.AboutRequest{})
	require.NoError(t, err)
	assert.Equal(t, exe, about.GetExecutable())
	assert.Equal(t, version.GetVersion(), about.GetVersion())

	_, err = client.Run(ctx, &pulumirpc.RunRequest{})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestLanguageHost_RunPluginRejects(t *testing.T) {
	t.Parallel()

	client, _ := startLanguageHost(t, LanguageHostOptions{Executable: "/nonexistent", AnalyzerName: "name"})
	dir := t.TempDir()
	tests := []struct {
		name string
		req  *pulumirpc.RunPluginRequest
	}{
		{
			name: "no args",
			req: &pulumirpc.RunPluginRequest{
				Info: &pulumirpc.ProgramInfo{ProgramDirectory: dir},
				Kind: "analyzer",
			},
		},
		{
			name: "nil info",
			req: &pulumirpc.RunPluginRequest{
				Args: []string{"127.0.0.1:1"},
				Kind: "analyzer",
			},
		},
		{
			name: "relative program directory",
			req: &pulumirpc.RunPluginRequest{
				Args: []string{"127.0.0.1:1"},
				Info: &pulumirpc.ProgramInfo{ProgramDirectory: "relative"},
				Kind: "analyzer",
			},
		},
		{
			name: "kind other than analyzer",
			req: &pulumirpc.RunPluginRequest{
				Args: []string{"127.0.0.1:1"},
				Info: &pulumirpc.ProgramInfo{ProgramDirectory: dir},
				Kind: "resource",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stream, err := client.RunPlugin(context.Background(), tt.req)
			require.NoError(t, err)
			_, err = stream.Recv()
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

func TestLanguageHost_RunPluginReportsChild(t *testing.T) {
	t.Parallel()

	client, _ := startLanguageHost(t, LanguageHostOptions{
		Executable:   testExecutable(t),
		AnalyzerName: analyzerName(),
	})
	dir := t.TempDir()
	const engine = "127.0.0.1:9999"
	const sentinel = "seen"
	stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
		Args: []string{engine},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: dir},
		Kind: "analyzer",
		Env: []string{
			"FINFOCUS_LANGHOST_TEST_CHILD=report",
			"FINFOCUS_LANGHOST_SENTINEL=" + sentinel,
		},
	})
	require.NoError(t, err)

	got := readPluginStream(stream)
	require.NoError(t, got.err)
	assert.Nil(t, got.exit)
	assert.Empty(t, got.stderr)

	text := string(got.stdout)
	line, rest, found := strings.Cut(text, "\n")
	require.True(t, found)
	assert.Equal(t, "line-one", line)

	var payload struct {
		Args0    string   `json:"args0"`
		Args     []string `json:"args"`
		WD       string   `json:"wd"`
		Sentinel string   `json:"sentinel"`
	}
	require.NoError(t, json.Unmarshal([]byte(rest), &payload))
	assert.Equal(t, analyzerName(), payload.Args0)
	assert.Equal(t, []string{analyzerName(), engine, "."}, payload.Args)
	assert.Equal(t, dir, payload.WD)
	assert.Equal(t, sentinel, payload.Sentinel)
}

func TestLanguageHost_RunPluginProjectDir(t *testing.T) {
	t.Parallel()

	ambient := os.Getenv("FINFOCUS_PROJECT_DIR")
	project := t.TempDir()
	wantProject := project
	if ambient != "" {
		wantProject = ambient
	}
	notDir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notDir, nil, 0o600))

	tests := []struct {
		name string
		pwd  string
		env  []string
		want string
	}{
		{name: "pwd becomes the project directory", pwd: project, want: wantProject},
		{name: "empty pwd leaves the environment alone", pwd: "", want: ambient},
		{name: "relative pwd is ignored", pwd: "relative/project", want: ambient},
		{name: "missing pwd is ignored", pwd: filepath.Join(project, "missing"), want: ambient},
		{name: "pwd that is a file is ignored", pwd: notDir, want: ambient},
		{
			name: "explicit request env wins",
			pwd:  project,
			env:  []string{"FINFOCUS_PROJECT_DIR=/explicit"},
			want: "/explicit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := startLanguageHost(t, LanguageHostOptions{
				Executable:   testExecutable(t),
				AnalyzerName: analyzerName(),
			})
			stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
				Args: []string{"127.0.0.1:9999"},
				Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
				Kind: "analyzer",
				Pwd:  tt.pwd,
				Env:  append([]string{"FINFOCUS_LANGHOST_TEST_CHILD=report"}, tt.env...),
			})
			require.NoError(t, err)

			got := readPluginStream(stream)
			require.NoError(t, got.err)
			_, rest, found := strings.Cut(string(got.stdout), "\n")
			require.True(t, found)

			var payload struct {
				Project string `json:"project"`
			}
			require.NoError(t, json.Unmarshal([]byte(rest), &payload))
			assert.Equal(t, tt.want, payload.Project)
		})
	}
}

func TestLanguageHost_RunPluginWorkingDirectory(t *testing.T) {
	t.Parallel()

	project := t.TempDir()
	tests := []struct {
		name string
		pwd  string
		want func(pack string) string
	}{
		{name: "valid pwd is the working directory", pwd: project, want: func(string) string { return project }},
		{name: "empty pwd keeps the pack directory", pwd: "", want: func(pack string) string { return pack }},
		{
			name: "relative pwd keeps the pack directory",
			pwd:  "relative",
			want: func(pack string) string { return pack },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := startLanguageHost(t, LanguageHostOptions{
				Executable:   testExecutable(t),
				AnalyzerName: analyzerName(),
			})
			pack := t.TempDir()
			stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
				Args: []string{"127.0.0.1:9999"},
				Info: &pulumirpc.ProgramInfo{ProgramDirectory: pack},
				Kind: "analyzer",
				Pwd:  tt.pwd,
				Env:  []string{"FINFOCUS_LANGHOST_TEST_CHILD=report"},
			})
			require.NoError(t, err)

			got := readPluginStream(stream)
			require.NoError(t, got.err)
			_, rest, found := strings.Cut(string(got.stdout), "\n")
			require.True(t, found)

			var payload struct {
				WD string `json:"wd"`
			}
			require.NoError(t, json.Unmarshal([]byte(rest), &payload))
			assert.Equal(t, tt.want(pack), payload.WD)
		})
	}
}

// Not parallel: t.Setenv changes the process environment.
func TestProjectDirEnv_HostEnvironmentWins(t *testing.T) {
	t.Setenv("FINFOCUS_PROJECT_DIR", "/from-host")

	assert.Nil(t, projectDirEnv(&pulumirpc.RunPluginRequest{Pwd: t.TempDir()}))
}

func TestLanguageHost_RunPluginStderr(t *testing.T) {
	t.Parallel()

	client, _ := startLanguageHost(t, LanguageHostOptions{
		Executable:   testExecutable(t),
		AnalyzerName: analyzerName(),
	})
	stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
		Env:  []string{"FINFOCUS_LANGHOST_TEST_CHILD=stderr"},
	})
	require.NoError(t, err)

	got := readPluginStream(stream)
	require.NoError(t, got.err)
	assert.Empty(t, got.stdout)
	assert.Contains(t, string(got.stderr), "err-line")
	assert.Nil(t, got.exit)
}

func TestLanguageHost_RunPluginExitCode(t *testing.T) {
	t.Parallel()

	client, _ := startLanguageHost(t, LanguageHostOptions{
		Executable:   testExecutable(t),
		AnalyzerName: analyzerName(),
	})
	stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
		Env:  []string{"FINFOCUS_LANGHOST_TEST_CHILD=exit3"},
	})
	require.NoError(t, err)

	got := readPluginStream(stream)
	require.NoError(t, got.err)
	require.NotNil(t, got.exit)
	assert.Equal(t, int32(3), *got.exit)
}

func TestLanguageHost_RunPluginMissingExecutable(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "missing-finfocus")
	client, _ := startLanguageHost(t, LanguageHostOptions{
		Executable:   missing,
		AnalyzerName: analyzerName(),
	})
	stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
	})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.Error(t, err)
}
