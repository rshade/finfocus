package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestDetectProjectSourceNearerMarker(t *testing.T) {
	t.Run("pulumi wins in the same directory", func(t *testing.T) {
		dir := t.TempDir()
		writeMarker(t, dir, "Pulumi.yaml")
		writeMarker(t, dir, "main.tf")
		t.Chdir(dir)
		got, err := cli.DetectProjectSource(".")
		require.NoError(t, err)
		assert.Equal(t, cli.ProjectKindPulumi, got.Kind)
	})

	t.Run("terraform wins over a parent pulumi project", func(t *testing.T) {
		parent := t.TempDir()
		writeMarker(t, parent, "Pulumi.yaml")
		dir := filepath.Join(parent, "stack")
		require.NoError(t, os.Mkdir(dir, 0o700))
		writeMarker(t, dir, "main.tf")
		t.Chdir(dir)
		got, err := cli.DetectProjectSource(".")
		require.NoError(t, err)
		assert.Equal(t, cli.ProjectKindTerraform, got.Kind)
	})

	t.Run("parent pulumi project without terraform files", func(t *testing.T) {
		parent := t.TempDir()
		writeMarker(t, parent, "Pulumi.yaml")
		dir := filepath.Join(parent, "child")
		require.NoError(t, os.Mkdir(dir, 0o700))
		t.Chdir(dir)
		got, err := cli.DetectProjectSource(".")
		require.NoError(t, err)
		assert.Equal(t, cli.ProjectKindPulumi, got.Kind)
	})

	t.Run("neither marker names both", func(t *testing.T) {
		t.Chdir(t.TempDir())
		_, err := cli.DetectProjectSource(".")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Pulumi")
		assert.Contains(t, err.Error(), "Terraform")
	})

	t.Run("tf json counts", func(t *testing.T) {
		dir := t.TempDir()
		writeMarker(t, dir, "main.tf.json")
		t.Chdir(dir)
		got, err := cli.DetectProjectSource(".")
		require.NoError(t, err)
		assert.Equal(t, cli.ProjectKindTerraform, got.Kind)
	})

	t.Run("terraform file only in a subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "nested")
		require.NoError(t, os.Mkdir(nested, 0o700))
		writeMarker(t, nested, "main.tf")
		t.Chdir(dir)
		_, err := cli.DetectProjectSource(".")
		require.Error(t, err)
		assert.ErrorIs(t, err, cli.ErrNoProjectMarker)
	})
}

func writeMarker(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("name: demo\n"), 0o600))
}
