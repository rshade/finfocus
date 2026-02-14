package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewAnalyzerUninstallCmd_RemovesInstallation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Install first
	installCmd := cli.NewAnalyzerInstallCmd()
	installCmd.SetOut(&bytes.Buffer{})
	installCmd.SetErr(&bytes.Buffer{})
	installCmd.SetArgs([]string{"--target-dir", dir})
	require.NoError(t, installCmd.Execute())

	// Uninstall
	cmd := cli.NewAnalyzerUninstallCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", dir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer uninstalled successfully")
	assert.Contains(t, output, "Removed:")
}

func TestNewAnalyzerUninstallCmd_NotInstalled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := cli.NewAnalyzerUninstallCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", dir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
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

	// Install first
	installCmd := cli.NewAnalyzerInstallCmd()
	installCmd.SetOut(&bytes.Buffer{})
	installCmd.SetErr(&bytes.Buffer{})
	installCmd.SetArgs([]string{"--target-dir", customDir})
	require.NoError(t, installCmd.Execute())

	// Uninstall from custom dir
	cmd := cli.NewAnalyzerUninstallCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", customDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer uninstalled successfully")

	// Verify directory is empty of analyzer dirs
	entries, err := os.ReadDir(customDir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, entry.IsDir() && len(entry.Name()) > 20 && entry.Name()[:19] == "analyzer-finfocus-v",
			"expected no analyzer directories to remain")
	}
}
