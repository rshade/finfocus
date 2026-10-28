package cli_test

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

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

type languageHostRun struct {
	cancel func()
	buf    *lockedBuffer
	errCh  <-chan error
	client pulumirpc.LanguageRuntimeClient
	ctx    context.Context
}

func startAnalyzerLanguageHost(t *testing.T) languageHostRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	buf := &lockedBuffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetContext(ctx)
	errCh := make(chan error, 1)
	go func() {
		errCh <- cli.RunAnalyzerLanguageHost(cmd)
	}()

	deadline := time.Now().Add(5 * time.Second)
	var text string
	for time.Now().Before(deadline) {
		text = buf.String()
		if strings.Contains(text, "\n") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	line, rest, found := strings.Cut(text, "\n")
	require.True(t, found, "port line, have %q", text)
	require.Empty(t, rest)
	_, err := strconv.Atoi(line)
	require.NoError(t, err)

	conn, err := grpc.NewClient(
		net.JoinHostPort("127.0.0.1", line),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return languageHostRun{
		cancel: cancel,
		buf:    buf,
		errCh:  errCh,
		client: pulumirpc.NewLanguageRuntimeClient(conn),
		ctx:    ctx,
	}
}

func requireHostStopped(t *testing.T, run languageHostRun) {
	t.Helper()
	started := time.Now()
	run.cancel()
	select {
	case err := <-run.errCh:
		require.NoError(t, err)
		assert.Less(t, time.Since(started), 4500*time.Millisecond)
	case <-time.After(6 * time.Second):
		t.Fatal("RunAnalyzerLanguageHost did not return")
	}
	line, rest, found := strings.Cut(run.buf.String(), "\n")
	require.True(t, found)
	require.NotEmpty(t, line)
	assert.Empty(t, rest)
}

// RunAnalyzerLanguageHost installs process-wide signal handlers.
//
//nolint:paralleltest // signal.NotifyContext is process-wide
func TestRunAnalyzerLanguageHost_PrintsPortAndStops(t *testing.T) {
	run := startAnalyzerLanguageHost(t)
	info, err := run.client.GetPluginInfo(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, version.GetVersion(), info.GetVersion())
	requireHostStopped(t, run)
}

// RunAnalyzerLanguageHost installs process-wide signal handlers.
//
//nolint:paralleltest // signal.NotifyContext is process-wide
func TestRunAnalyzerLanguageHost_StopWithIgnoringChild(t *testing.T) {
	run := startAnalyzerLanguageHost(t)
	stream, err := run.client.RunPlugin(run.ctx, &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
		Env:  []string{"FINFOCUS_LANGHOST_TEST_CHILD=ignore-sigint"},
	})
	require.NoError(t, err)

	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.Contains(t, string(msg.GetStdout()), "ready")
	requireHostStopped(t, run)
}
