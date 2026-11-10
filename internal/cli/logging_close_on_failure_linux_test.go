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
//nolint:paralleltest // isolateConfig sets the environment, and the test counts process-wide descriptors.
func TestFailedCommandReleasesItsLogFile(t *testing.T) {
	isolateConfig(t)
	isolateFromPulumiProject(t)
	logPath := filepath.Join(os.Getenv("FINFOCUS_HOME"), "logs", "finfocus.log")

	ok := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), []string{"config", "list"})
	require.Equal(t, 0, ok.ExitCode, string(ok.Stderr))
	require.FileExists(t, logPath, "the command wrote to the log file this test counts handles on")
	assert.Zero(t, openHandlesTo(t, logPath), "a successful command closes its log file")

	failed := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"config", "set", "invalid.key", "value"})
	require.NotEqual(t, 0, failed.ExitCode)
	assert.Zero(t, openHandlesTo(t, logPath), "a failing command closes its log file too")
}

// The cost group validates its configuration in PersistentPreRunE, after the
// root hook has opened the log file, so RunE never runs when validation fails.
//
//nolint:paralleltest // isolateConfig sets the environment, and the test counts process-wide descriptors.
func TestFailedPreRunReleasesItsLogFile(t *testing.T) {
	isolateConfig(t)
	isolateFromPulumiProject(t)
	home := os.Getenv("FINFOCUS_HOME")
	logPath := filepath.Join(home, "logs", "finfocus.log")
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.hujson"),
		[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"maybe"}}}}`), 0o600))

	failed := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "actual", "--from", "2026-10-01", "--to", "2026-10-02"})
	require.Equal(t, 2, failed.ExitCode, string(failed.Stderr))
	require.FileExists(t, logPath, "the pre-run opened the log file before validation failed")
	assert.Zero(t, openHandlesTo(t, logPath), "a command whose pre-run fails closes its log file")
}
