package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

func TestNewAnalyzerInstallCmd_FreshInstall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cmd := cli.NewAnalyzerInstallCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", dir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer installed successfully")
	assert.Contains(t, output, "Version: "+version.GetVersion())
	assert.Contains(t, output, "Path:")
	assert.Contains(t, output, "Method:")
}

func TestNewAnalyzerInstallCmd_AlreadyInstalled_SameVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Install first
	cmd1 := cli.NewAnalyzerInstallCmd()
	cmd1.SetOut(&bytes.Buffer{})
	cmd1.SetErr(&bytes.Buffer{})
	cmd1.SetArgs([]string{"--target-dir", dir})
	require.NoError(t, cmd1.Execute())

	// Install again
	cmd2 := cli.NewAnalyzerInstallCmd()
	var buf bytes.Buffer
	cmd2.SetOut(&buf)
	cmd2.SetErr(&buf)
	cmd2.SetArgs([]string{"--target-dir", dir})

	err := cmd2.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer already installed")
	assert.Contains(t, output, "Use --force to reinstall")
}

func TestNewAnalyzerInstallCmd_AlreadyInstalled_DifferentVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Simulate old version
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.0.1"), 0o755))

	cmd := cli.NewAnalyzerInstallCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", dir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer already installed at v0.0.1")
	assert.Contains(t, output, "Current finfocus version: v"+version.GetVersion())
	assert.Contains(t, output, "Use --force to upgrade")
}

func TestNewAnalyzerInstallCmd_ForceReinstall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Simulate old version
	oldDir := filepath.Join(dir, "analyzer-finfocus-v0.0.1")
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "pulumi-analyzer-finfocus"), []byte("old"), 0o755))

	cmd := cli.NewAnalyzerInstallCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", dir, "--force"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Analyzer installed successfully")
}

func TestNewAnalyzerInstallCmd_FlagParsing(t *testing.T) {
	t.Parallel()

	cmd := cli.NewAnalyzerInstallCmd()

	// Verify flags exist
	forceFlag := cmd.Flags().Lookup("force")
	require.NotNil(t, forceFlag)
	assert.Equal(t, "false", forceFlag.DefValue)

	targetFlag := cmd.Flags().Lookup("target-dir")
	require.NotNil(t, targetFlag)
	assert.Equal(t, "", targetFlag.DefValue)
}

func TestNewAnalyzerInstallCmd_TargetDirPropagation(t *testing.T) {
	t.Parallel()

	customDir := filepath.Join(t.TempDir(), "custom-location")
	cmd := cli.NewAnalyzerInstallCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", customDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, customDir)
}

func TestNewAnalyzerInstallCmd_ErrorOnInvalidDir(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission test")
	}

	// Create a read-only directory so MkdirAll fails inside it
	parentDir := t.TempDir()
	readOnlyDir := filepath.Join(parentDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o555))
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})

	cmd := cli.NewAnalyzerInstallCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--target-dir", filepath.Join(readOnlyDir, "nested")})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "install analyzer")
}
