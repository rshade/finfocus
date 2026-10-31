package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestCostActualTerraformRefusesToPlan(t *testing.T) {
	marker := installTerraformRefusalSentinel(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# fixture\n"), 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(dir)

	result := axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"cost", "actual"},
	)
	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "--terraform-state")
	assert.Contains(t, string(result.Stderr), "does not run a plan")
	_, err := os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCostHistoryCollectTerraformRefuses(t *testing.T) {
	marker := installTerraformRefusalSentinel(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# fixture\n"), 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(dir)

	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "history", "collect", "--stack", "dev"})
	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "Pulumi checkpoints")
	assert.Contains(t, string(result.Stderr), "Terraform")
	_, err := os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func installTerraformRefusalSentinel(t *testing.T) string {
	requirePOSIXShell(t)
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "binary-was-run")
	t.Setenv("FINFOCUS_TEST_TERRAFORM_MARKER", marker)
	script := []byte(`#!/bin/sh
echo ran > "$FINFOCUS_TEST_TERRAFORM_MARKER"
exit 1
`)
	for _, name := range []string{"terraform", "tofu", "pulumi"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), script, 0o700))
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}
