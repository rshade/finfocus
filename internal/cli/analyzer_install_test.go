package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

func TestNewAnalyzerInstallCmd_FreshInstall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := cli.NewRootCmd("test-version")

	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
	assert.Contains(t, output, "Analyzer installed successfully")
	assert.Contains(t, output, "Version: v"+version.GetVersion())
	assert.Contains(t, output, "Path:")
	assert.Contains(t, output, "Method:")
}

func TestNewAnalyzerInstallCmd_AlreadyInstalled_SameVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := cli.NewRootCmd("test-version")

	// Install first
	result1 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result1.ExitCode)

	// Install again
	result2 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result2.ExitCode)

	output := string(result2.Stdout)
	assert.Contains(t, output, "Analyzer already installed")
	assert.Contains(t, output, "Use --force to reinstall")
}

func TestNewAnalyzerInstallCmd_AlreadyInstalled_DifferentVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Simulate old version
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "analyzer-finfocus-v0.0.1"), 0o755))

	root := cli.NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
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
	// Binary name must match analyzer.analyzerBinaryName (unexported, so not
	// referenceable from this external _test package). Keep in sync manually.
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "pulumi-analyzer-finfocus"), []byte("old"), 0o755))

	root := cli.NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir, "--force"})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
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
	root := cli.NewRootCmd("test-version")

	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", customDir})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
	assert.Contains(t, output, customDir)
}

func TestAnalyzerInstallCmd_PrintsPATHInstructions(t *testing.T) {
	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	root := cli.NewRootCmd("test-version")
	result := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result.ExitCode)

	output := string(result.Stdout)
	policyPackDir := filepath.Join(finfocusHome, "analyzer")

	// Generic message should appear regardless of OS
	assert.Contains(t, output, "To use the analyzer with pulumi preview")
	assert.Contains(t, output, "pulumi preview --policy-pack \""+policyPackDir+"\"")

	// PATH instruction format depends on OS
	if runtime.GOOS == "windows" {
		assert.Contains(t, output, "$env:PATH")
	} else {
		assert.Contains(t, output, "export PATH=\""+policyPackDir+":$PATH\"")
	}
}

func TestAnalyzerInstallCmd_NoPATHOnNoOp(t *testing.T) {
	dir := t.TempDir()
	finfocusHome := t.TempDir()
	t.Setenv("FINFOCUS_HOME", finfocusHome)

	root := cli.NewRootCmd("test-version")

	// First install
	result1 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result1.ExitCode)

	// Second install (no-op)
	result2 := axtest.Run(context.Background(), t, root, []string{"analyzer", "install", "--target-dir", dir})
	require.Equal(t, 0, result2.ExitCode)

	output := string(result2.Stdout)
	assert.NotContains(t, output, "To use the analyzer with pulumi preview")
	assert.NotContains(t, output, "export PATH=")
	assert.NotContains(t, output, "pulumi preview --policy-pack")
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

	root := cli.NewRootCmd("test-version")
	result := axtest.Run(
		context.Background(),
		t,
		root,
		[]string{"analyzer", "install", "--target-dir", filepath.Join(readOnlyDir, "nested")},
	)
	require.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "install analyzer")
}
