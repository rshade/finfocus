package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o750))
	p := filepath.Join(dir, AllocationPolicyFile)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

func isolatePolicyDirs(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	project := filepath.Join(t.TempDir(), ".finfocus")
	SetResolvedProjectDir("")
	t.Cleanup(func() { SetResolvedProjectDir("") })
	return home, project
}

func TestResolveAllocationPolicy_Precedence(t *testing.T) {
	home, project := isolatePolicyDirs(t)
	ctx := context.Background()

	got, err := ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Nil(t, got.JSON, "no file → plugin defaults")
	assert.Empty(t, got.Source)

	globalPath := writePolicy(t, home, `{ "version": 1, /* global */ }`)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, globalPath, got.Source)
	assert.JSONEq(t, `{"version":1}`, string(got.JSON), "hujson comments and trailing commas standardized")

	projectPath := writePolicy(t, project, `{"idle": "separate"}`)
	SetResolvedProjectDir(project)
	got, err = ResolveAllocationPolicy(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, projectPath, got.Source)
	assert.JSONEq(t, `{"idle":"separate"}`, string(got.JSON), "project replaces global, no merge")

	flagPath := writePolicy(t, t.TempDir(), `{"version": 1}`)
	got, err = ResolveAllocationPolicy(ctx, flagPath)
	require.NoError(t, err)
	assert.Equal(t, flagPath, got.Source)
}

func TestResolveAllocationPolicy_NullDocument(t *testing.T) {
	home, _ := isolatePolicyDirs(t)

	path := writePolicy(t, home, "null\n")
	got, err := ResolveAllocationPolicy(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, path, got.Source)
	assert.Nil(t, got.JSON, "a null document is equivalent to no policy")
}

func TestResolveAllocationPolicy_Errors(t *testing.T) {
	home, _ := isolatePolicyDirs(t)
	ctx := context.Background()

	_, err := ResolveAllocationPolicy(ctx, filepath.Join(t.TempDir(), "missing.hujson"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.hujson")

	broken := writePolicy(t, home, `{"version": }`)
	_, err = ResolveAllocationPolicy(ctx, "")
	require.Error(t, err, "a discovered broken file never falls back to defaults")
	assert.Contains(t, err.Error(), broken)

	commentOnly := writePolicy(t, t.TempDir(), "// nothing yet\n")
	_, err = ResolveAllocationPolicy(ctx, commentOnly)
	require.Error(t, err, "a comment-only file is a parse error, not an empty policy")
	assert.Contains(t, err.Error(), commentOnly)
}
