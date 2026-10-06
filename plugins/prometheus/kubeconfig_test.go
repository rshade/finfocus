package prometheus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenKubeconfig reads KUBECONFIG, which is process-wide, so this test
// stays sequential.
func TestOpenKubeconfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")
	const host = "https://127.0.0.1:6443"
	body := []byte(`apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: ` + host + `
contexts:
- name: kind-test
  context:
    cluster: c
    user: u
current-context: kind-test
users:
- name: u
  user:
    token: test-token
`)
	require.NoError(t, os.WriteFile(path, body, 0o600))
	t.Setenv("KUBECONFIG", path)

	client, gotHost, err := openKubeconfig("")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, host, gotHost)

	client, gotHost, err = openKubeconfig("kind-test")
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, host, gotHost)

	_, _, err = openKubeconfig("missing")
	require.Error(t, err)
}
