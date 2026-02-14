package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/pkg/version"
)

func TestIntegration_InstallVerifyUninstallCycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Step 1: Verify not installed
	installed, err := analyzer.IsInstalled(dir)
	require.NoError(t, err)
	assert.False(t, installed, "expected analyzer to not be installed initially")

	// Step 2: Install
	result, err := analyzer.Install(ctx, analyzer.InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Installed)
	assert.Equal(t, version.GetVersion(), result.Version)
	assert.NotEmpty(t, result.Method)

	// Step 3: Verify installed
	installed, err = analyzer.IsInstalled(dir)
	require.NoError(t, err)
	assert.True(t, installed, "expected analyzer to be installed")

	ver, err := analyzer.InstalledVersion(dir)
	require.NoError(t, err)
	assert.Equal(t, version.GetVersion(), ver)

	// Step 4: Verify binary exists on disk
	_, statErr := os.Lstat(result.Path)
	require.NoError(t, statErr, "expected installed binary to exist")

	// Step 5: Verify idempotent re-install (no-op)
	result2, err := analyzer.Install(ctx, analyzer.InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.True(t, result2.Installed)
	assert.Empty(t, result2.Method, "expected no action on re-install")

	// Step 6: Uninstall
	err = analyzer.Uninstall(ctx, dir)
	require.NoError(t, err)

	// Step 7: Verify uninstalled
	installed, err = analyzer.IsInstalled(dir)
	require.NoError(t, err)
	assert.False(t, installed, "expected analyzer to be uninstalled")

	// Step 8: Verify idempotent uninstall (no-op)
	err = analyzer.Uninstall(ctx, dir)
	require.NoError(t, err)
}

func TestIntegration_ForceUpgradeCycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ctx := context.Background()

	// Simulate old version
	oldDir := filepath.Join(dir, "analyzer-finfocus-v0.0.1-old")
	require.NoError(t, os.MkdirAll(oldDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(oldDir, "pulumi-analyzer-finfocus"),
		[]byte("old-binary"),
		0o755,
	))

	// Verify needs update
	needsUpdate, err := analyzer.NeedsUpdate(dir)
	require.NoError(t, err)
	assert.True(t, needsUpdate)

	// Install without force - should report status
	result, err := analyzer.Install(ctx, analyzer.InstallOptions{TargetDir: dir})
	require.NoError(t, err)
	assert.True(t, result.NeedsUpdate)
	assert.Equal(t, "0.0.1-old", result.Version)

	// Force install - should replace
	result, err = analyzer.Install(ctx, analyzer.InstallOptions{TargetDir: dir, Force: true})
	require.NoError(t, err)
	assert.Equal(t, version.GetVersion(), result.Version)
	assert.NotEmpty(t, result.Method)

	// Old directory should be gone
	_, statErr := os.Stat(oldDir)
	assert.True(t, os.IsNotExist(statErr))

	// No longer needs update
	needsUpdate, err = analyzer.NeedsUpdate(dir)
	require.NoError(t, err)
	assert.False(t, needsUpdate)
}
