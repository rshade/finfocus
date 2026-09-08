package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewAnalyzerUninstallCmd_RemovesInstallation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := cli.NewRootCmd("test-version")

	// Install first
	result1 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result1.ExitCode)

	// Uninstall
	result2 := axtest.Run(context.Background(), t, root, []string{"analyzer", "uninstall", "--target-dir", dir})
	require.Equal(t, 0, result2.ExitCode)

	output := string(result2.Stdout)
	assert.Contains(t, output, "Analyzer uninstalled successfully")
	assert.Contains(t, output, "Removed:")
}

func TestNewAnalyzerUninstallCmd_NotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := cli.NewRootCmd("test-version")

	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "uninstall", "--target-dir", dir})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
	assert.Contains(t, output, "Analyzer is not installed")
}

func TestNewAnalyzerUninstallCmd_FlagParsing(t *testing.T) {
	t.Parallel()

	cmd := cli.NewAnalyzerUninstallCmd()

	targetFlag := cmd.Flags().Lookup("target-dir")
	require.NotNil(t, targetFlag)
	assert.Equal(t, "", targetFlag.DefValue)
}

func TestNewAnalyzerUninstallCmd_TargetDirPropagation(t *testing.T) {
	t.Parallel()

	customDir := filepath.Join(t.TempDir(), "custom-location")
	root := cli.NewRootCmd("test-version")

	// Install first
	result1 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", customDir})
	require.Equal(t, 0, result1.ExitCode)

	// Uninstall from custom dir
	result2 := axtest.Run(context.Background(), t, root, []string{"analyzer", "uninstall", "--target-dir", customDir})
	require.Equal(t, 0, result2.ExitCode)

	output := string(result2.Stdout)
	assert.Contains(t, output, "Analyzer uninstalled successfully")

	// Verify directory is empty of analyzer dirs
	entries, err := os.ReadDir(customDir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, entry.IsDir() && strings.HasPrefix(entry.Name(), "analyzer-finfocus-v"),
			"expected no analyzer directories to remain")
	}
}
