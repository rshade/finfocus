//go:build linux

package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/ax-go/axtest"

	"github.com/rshade/finfocus/internal/cli"
)

// openHandlesTo counts this process's open file descriptors that point at path.
func openHandlesTo(t *testing.T, path string) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		target, linkErr := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if linkErr == nil && strings.TrimSuffix(target, " (deleted)") == path {
			count++
		}
	}
	return count
}

// A command that fails must release its log file like one that succeeds:
// cobra skips PersistentPostRunE after an error, and Windows cannot remove a
// directory that holds an open file.
//
// Not parallel: it sets the process environment and counts process-wide descriptors.
func TestFailedCommandReleasesItsLogFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	logPath := filepath.Join(home, "logs", "finfocus.log")

	ok := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "list"})
	require.Equal(t, 0, ok.ExitCode, string(ok.Stderr))
	assert.Zero(t, openHandlesTo(t, logPath), "a successful command closes its log file")

	failed := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "invalid.key", "value"})
	require.NotEqual(t, 0, failed.ExitCode)
	assert.Zero(t, openHandlesTo(t, logPath), "a failing command closes its log file too")
}
