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

func TestSelectedStackUsesTheActiveBackend(t *testing.T) {
	const cloud, selfHosted = "https://api.pulumi.com", "https://pulumi.example.com"
	workspace := `{"stacks":{"` + cloud + `":"cloud-dev","` + selfHosted + `":"hosted-dev"}}`

	cases := []struct {
		name        string
		env         string
		projectYAML string
		current     string
		want        string
		wantErr     bool
	}{
		{name: "environment wins", env: selfHosted, current: cloud, want: "hosted-dev"},
		{
			name:        "project backend",
			projectYAML: "backend:\n  url: " + selfHosted + "\n",
			current:     cloud,
			want:        "hosted-dev",
		},
		{name: "current login", current: selfHosted, want: "hosted-dev"},
		{name: "trailing slash", current: selfHosted + "/", want: "hosted-dev"},
		{name: "default is Pulumi Cloud", want: "cloud-dev"},
		{name: "no entry for the active backend", env: "https://other.example.com", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("PULUMI_HOME", home)
			t.Setenv("PULUMI_CREDENTIALS_PATH", "")
			t.Setenv("PULUMI_BACKEND_URL", tc.env)
			if tc.current != "" {
				require.NoError(t, os.WriteFile(
					filepath.Join(home, "credentials.json"), []byte(`{"current":"`+tc.current+`"}`), 0o600))
			}
			project := t.TempDir()
			require.NoError(t, os.WriteFile(
				filepath.Join(project, "Pulumi.yaml"), []byte("name: demo\n"+tc.projectYAML), 0o600))
			abs, err := filepath.Abs(filepath.Join(project, "Pulumi.yaml"))
			require.NoError(t, err)
			path := filepath.Join(home, "workspaces", fmt.Sprintf("demo-%x-workspace.json", sha1.Sum([]byte(abs))))
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
			require.NoError(t, os.WriteFile(path, []byte(workspace), 0o600))

			got, err := SelectedStack(project)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
