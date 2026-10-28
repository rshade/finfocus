//go:build unix

package analyzer

import (
	"context"
	"os"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLanguageHost_RunPluginCancelSIGINT(t *testing.T) {
	t.Parallel()

	const wait = 2 * time.Second
	client, _ := startLanguageHost(t, LanguageHostOptions{
		Executable:   testExecutable(t),
		AnalyzerName: analyzerName(),
		WaitDelay:    wait,
	})
	done := t.TempDir() + "/exited"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.RunPlugin(ctx, &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
		Env: []string{
			"FINFOCUS_LANGHOST_TEST_CHILD=wait-signal",
			"FINFOCUS_LANGHOST_TEST_DONE=" + done,
		},
	})
	require.NoError(t, err)

	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.Contains(t, string(msg.GetStdout()), "ready")

	cancel()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(done); statErr == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("child did not exit cleanly within WaitDelay")
}

func TestLanguageHost_RunPluginShutdownCancelsChild(t *testing.T) {
	t.Parallel()

	const wait = 2 * time.Second
	client, host := startLanguageHost(t, LanguageHostOptions{
		Executable:   testExecutable(t),
		AnalyzerName: analyzerName(),
		WaitDelay:    wait,
	})
	done := t.TempDir() + "/exited"
	stream, err := client.RunPlugin(context.Background(), &pulumirpc.RunPluginRequest{
		Args: []string{"127.0.0.1:1"},
		Info: &pulumirpc.ProgramInfo{ProgramDirectory: t.TempDir()},
		Kind: "analyzer",
		Env: []string{
			"FINFOCUS_LANGHOST_TEST_CHILD=wait-signal",
			"FINFOCUS_LANGHOST_TEST_DONE=" + done,
		},
	})
	require.NoError(t, err)

	msg, err := stream.Recv()
	require.NoError(t, err)
	assert.Contains(t, string(msg.GetStdout()), "ready")

	host.Shutdown()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if _, statErr := os.Stat(done); statErr == nil {
			got := readPluginStream(stream)
			require.NoError(t, got.err)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("child did not exit after host shutdown")
}
