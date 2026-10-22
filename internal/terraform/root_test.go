package terraform

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasRoot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"main.tf", true}, {"main.tf.json", true}, {"terraform.tfvars", false}, {"main.tf.bak", false}, {"README.md", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, tc.name), nil, 0o600))
			got, err := HasRoot(dir)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "module.tf"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "module.tf", "child.tf"), nil, 0o600))
	got, err := HasRoot(dir)
	require.NoError(t, err)
	assert.False(t, got)
	_, err = HasRoot(filepath.Join(dir, "missing"))
	require.Error(t, err)
}
