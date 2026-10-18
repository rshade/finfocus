//go:build unix

package integration_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const mcpSignalExitBudget = 15 * time.Second

func TestMCPServer_ShutsDownCleanlyOnSignal(t *testing.T) {
	t.Parallel()

	env := newMCPTestEnv(t)

	for _, entry := range [][]string{{"mcp-server"}, {"--mcp"}} {
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
			t.Run(entry[0]+"/"+sig.String(), func(t *testing.T) {
				t.Parallel()

				s := startMCPSession(t, env, entry...)
				require.NotEmpty(t, s.toolNames())

				require.NoError(t, s.cmd.Process.Signal(sig))

				done := make(chan error, 1)
				go func() { done <- s.cmd.Wait() }()
				select {
				case err := <-done:
					require.NoError(t, err, "exit status after %s; stderr:\n%s", sig, s.stderr.String())
					assert.True(t, s.cmd.ProcessState.Success())
				case <-time.After(mcpSignalExitBudget):
					require.FailNow(
						t,
						"MCP server did not exit after signal",
						"%s; stderr:\n%s",
						sig,
						s.stderr.String(),
					)
				}
			})
		}
	}
}
