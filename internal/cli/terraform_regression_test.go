package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

//nolint:paralleltest,nolintlint // Changes process environment and executes a fake binary.
func TestPulumiProjectionDoesNotRunTerraform(t *testing.T) {
	dir := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(dir, "Pulumi.yaml"),
			[]byte("name: demo\nruntime: yaml\n"),
			0o600,
		),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(dir, "main.tf"), []byte("# sibling Terraform\n"), 0o600),
	)
	t.Chdir(dir)
	marker := filepath.Join(t.TempDir(), "ran")
	bin := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(bin, "terraform"),
			[]byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"),
			0o700,
		),
	)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{preview: []byte(`{"steps":[]}`)}))
	cmd := &cobra.Command{}
	cmd.Flags().String("stack", "dev", "")
	resources, unchanged, err := loadProjectedResources(
		context.Background(),
		cmd,
		costProjectedParams{},
		nil,
	)
	require.NoError(t, err)
	assert.Empty(t, resources)
	assert.False(t, unchanged)
	assert.NoFileExists(t, marker)
}

func TestSkippedTerraformProvidersOnlyWriteStderr(t *testing.T) {
	t.Parallel()
	var out, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	printSkippedProviders(cmd, map[string]int{"helm": 3, "random": 2})
	assert.Empty(t, out.String())
	assert.Equal(
		t,
		"Skipped 3 helm resources: no cost.\nSkipped 2 random resources: no cost.\n",
		stderr.String(),
	)
}
