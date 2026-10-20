package pulumi

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failRunner struct {
	t *testing.T
}

func (r failRunner) Run(context.Context, string, string, []string, ...string) ([]byte, []byte, error) {
	r.t.Fatal("pulumi CLI was invoked")
	return nil, nil, nil
}

func TestLoadStackContext(t *testing.T) {
	t.Cleanup(SetRunnerForTest(failRunner{t: t}))
	home := t.TempDir()
	t.Setenv("PULUMI_HOME", home)
	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(project, "Pulumi.yaml"),
			[]byte("name: demo\nbackend:\n  url: https://api.pulumi.com\n"),
			0o600,
		),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(project, "Pulumi.dev.yaml"), []byte("environment:\n  - proj/dev\n"), 0o600),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(project, "Pulumi.prod.yaml"), []byte("environment:\n  - proj/prod\n"), 0o600),
	)

	got, err := LoadStackContext(project, "acme/demo/dev")
	require.NoError(t, err)
	assert.Equal(t, "demo", got.ProjectName)
	assert.Equal(t, "https://api.pulumi.com", got.BackendURL)
	assert.Equal(t, "acme/demo/dev", got.Stack)
	assert.Equal(t, StackEnvImports, got.Environment.Form)
	assert.Equal(t, []string{"proj/dev"}, got.Environment.Imports)

	writeWorkspace(t, home, project, "demo", `{"stacks":{"https://api.pulumi.com":"prod"}}`)
	got, err = LoadStackContext(project, "")
	require.NoError(t, err)
	assert.Equal(t, "prod", got.Stack)
	assert.Equal(t, []string{"proj/prod"}, got.Environment.Imports)

	got, err = LoadStackContext(project, "dev")
	require.NoError(t, err)
	assert.Equal(t, "dev", got.Stack)
	assert.Equal(t, []string{"proj/dev"}, got.Environment.Imports)
}

func writeWorkspace(t *testing.T, home, project, name, body string) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(project, "Pulumi.yaml"))
	require.NoError(t, err)
	sum := sha1.Sum([]byte(abs))
	path := filepath.Join(home, "workspaces", fmt.Sprintf("%s-%x-workspace.json", name, sum))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}
