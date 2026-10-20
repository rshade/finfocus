package pulumi

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectedStackFormats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PULUMI_HOME", home)
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: demo\n"), 0o600))
	abs, err := filepath.Abs(filepath.Join(project, "Pulumi.yaml"))
	require.NoError(t, err)
	sum := sha1.Sum([]byte(abs))
	path := filepath.Join(home, "workspaces", fmt.Sprintf("demo-%x-workspace.json", sum))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))

	require.NoError(t, os.WriteFile(path, []byte(`{"stacks":{"https://api.pulumi.com":"acme/demo/dev"}}`), 0o600))
	got, err := SelectedStack(project)
	require.NoError(t, err)
	assert.Equal(t, "acme/demo/dev", got)

	require.NoError(t, os.WriteFile(path, []byte(`{"stack":"legacy"}`), 0o600))
	got, err = SelectedStack(project)
	require.NoError(t, err)
	assert.Equal(t, "legacy", got)

	require.NoError(t, os.Remove(path))
	_, err = SelectedStack(project)
	require.Error(t, err)

	require.NoError(t, os.WriteFile(path, []byte(`{`), 0o600))
	_, err = SelectedStack(project)
	require.Error(t, err)
}
