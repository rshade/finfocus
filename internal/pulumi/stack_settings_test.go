package pulumi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackSettingsNameCandidates(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"dev", "acme/infra/dev"}, StackSettingsNameCandidates("acme/infra/dev"))
	assert.Nil(t, StackSettingsNameCandidates("  "))
}

func TestReadStackSettingsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Pulumi.dev.yaml"), []byte("environment: []\n"), 0o600))
	got, err := ReadStackSettingsFile(dir, "acme/infra/dev")
	require.NoError(t, err)
	assert.Equal(t, "environment: []\n", string(got))
	_, err = ReadStackSettingsFile(dir, "  ")
	require.Error(t, err)
}

func TestParseStackEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
		form StackEnvForm
		refs []string
	}{
		{
			name: "list",
			yaml: "environment:\n  - proj/env\n  - other/env\n",
			form: StackEnvImports,
			refs: []string{"proj/env", "other/env"},
		},
		{
			name: "imports only",
			yaml: "environment:\n  imports:\n    - proj/env\n",
			form: StackEnvImports,
			refs: []string{"proj/env"},
		},
		{name: "missing", yaml: "config: {}\n", form: StackEnvAbsent},
		{name: "values", yaml: "environment:\n  values:\n    FOO: bar\n", form: StackEnvUnsupported},
		{
			name: "merge import",
			yaml: "environment:\n  imports:\n    - myenv:\n        merge: false\n",
			form: StackEnvUnsupported,
		},
		{
			name: "extra key",
			yaml: "environment:\n  imports:\n    - proj/env\n  values:\n    FOO: bar\n",
			form: StackEnvUnsupported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseStackEnvironment([]byte(tt.yaml))
			require.NoError(t, err)
			assert.Equal(t, tt.form, got.Form)
			assert.Equal(t, tt.refs, got.Imports)
		})
	}
}
