//go:build unix

package terraform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveIdentitySharedOrigin(t *testing.T) {
	t.Parallel()

	httpsDir := gitRepo(t, "https://github.com/org/repo.git")
	scpDir := gitRepo(t, "git@github.com:org/repo.git")
	httpsID, err := ResolveIdentity(context.Background(), httpsDir, false)
	require.NoError(t, err)
	scpID, err := ResolveIdentity(context.Background(), scpDir, false)
	require.NoError(t, err)
	assert.Equal(t, httpsID.Project, scpID.Project)
	assert.Equal(t, "unscoped", httpsID.Stack)
	assert.Equal(t, ".", httpsID.Label)
	assert.NotContains(t, httpsID.Label, "github.com")
	assert.NotContains(t, httpsID.Label, "git@")

	nested := filepath.Join(httpsDir, "stack")
	require.NoError(t, os.Mkdir(nested, 0o700))
	nestedID, err := ResolveIdentity(context.Background(), nested, false)
	require.NoError(t, err)
	assert.Equal(t, "stack", nestedID.Label)
	assert.NotEqual(t, httpsID.Project, nestedID.Project)
}

func TestResolveIdentityAbsolutePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	git(t, dir, "init")
	id, err := ResolveIdentity(context.Background(), dir, false)
	require.NoError(t, err)
	cleaned := filepath.ToSlash(dir)
	sum := sha256.Sum256([]byte("tf-id-abspath-v1\n" + cleaned))
	assert.Equal(t, hex.EncodeToString(sum[:]), id.Project)
	assert.Equal(t, "path:"+cleaned, id.Label)
	assert.Equal(t, "unscoped", id.Stack)
	assert.NotContains(t, id.Label, "git@")
}

func TestResolveIdentityWorkspaceOptIn(t *testing.T) {
	dir := gitRepo(t, "https://github.com/org/repo")
	off, err := ResolveIdentity(context.Background(), dir, false)
	require.NoError(t, err)

	bin := t.TempDir()
	marker := filepath.Join(dir, "called")
	script := "#!/bin/sh\nif [ \"$1\" = \"workspace\" ]; then echo dev; echo called > " +
		"'" + marker + "'; exit 0; fi\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	stillOff, err := ResolveIdentity(context.Background(), dir, false)
	require.NoError(t, err)
	assert.Equal(t, off, stillOff)
	_, statErr := os.Stat(marker)
	require.ErrorIs(t, statErr, os.ErrNotExist)

	on, err := ResolveIdentity(context.Background(), dir, true)
	require.NoError(t, err)
	assert.Equal(t, "dev", on.Stack)
	assert.Equal(t, ". dev", on.Label)
	assert.NotEqual(t, off.Project, on.Project)
	_, err = os.Stat(marker)
	require.NoError(t, err)

	unscoped := strings.Replace(script, "echo dev", "echo unscoped", 1)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(unscoped), 0o700))
	_, err = ResolveIdentity(context.Background(), dir, true)
	require.ErrorIs(t, err, ErrUnscopedWorkspace)
}

func gitRepo(t *testing.T, remote string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init")
	git(t, dir, "remote", "add", "origin", remote)
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestNormalizeRemoteVariants(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		remote, want string
		valid        bool
	}{
		{"https://user:password@GITHUB.COM/org/repo.git?secret=x#fragment", "github.com/org/repo", true},
		{"ssh://git@GitHub.com/org/repo.git", "github.com/org/repo", true},
		{"git@GITHUB.COM:org/repo.git", "github.com/org/repo", true},
		{"https://github.com/", "", false}, {"https:///nohost", "", false}, {"https://[invalid", "", false},
		{"/local/path", "", false}, {"host:org/repo", "", false}, {"git@:repo", "", false}, {"git@host:.git", "", false},
	} {
		t.Run(tc.remote, func(t *testing.T) {
			t.Parallel()
			got, ok := normalizeRemote(tc.remote)
			assert.Equal(t, tc.valid, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

//nolint:paralleltest // Writes and executes fake binaries and changes PATH.
func TestWorkspaceIdentityFailuresAndFallbacks(t *testing.T) {
	for _, tc := range []struct{ name, script, want string }{
		{"empty", "#!/bin/sh\nexit 0\n", "empty name"},
		{"failed", "#!/bin/sh\nexit 1\n", "reading OpenTofu workspace"},
		{"missing", "", "tofu command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			if tc.script != "" {
				require.NoError(
					t,
					os.WriteFile(filepath.Join(bin, "tofu"), []byte(tc.script), 0o700),
				)
			}
			t.Setenv("PATH", bin)
			_, err := ResolveIdentity(context.Background(), t.TempDir(), true)
			require.ErrorContains(t, err, tc.want)
		})
	}
	t.Run("workspace without origin", func(t *testing.T) {
		dir := t.TempDir()
		installFake(t, "#!/bin/sh\necho dev\n", nil)
		id, err := ResolveIdentity(context.Background(), dir, true)
		require.NoError(t, err)
		assert.Equal(t, "dev", id.Stack)
		assert.Equal(t, "path:"+filepath.ToSlash(dir)+" dev", id.Label)
	})
	t.Run("invalid origin falls back", func(t *testing.T) {
		dir := gitRepo(t, "/local/unsupported")
		id, err := ResolveIdentity(context.Background(), dir, false)
		require.NoError(t, err)
		assert.Equal(t, absoluteIdentity(dir, ""), id)
	})
}

func TestWorkspaceUsesOnlyOpenTofu(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "terraform-was-run")
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(bin, "terraform"),
			[]byte("#!/bin/sh\necho forbidden > "+shellQuote(marker)+"\necho forbidden\n"),
			0o700,
		),
	)
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(bin, "tofu"),
			[]byte(
				"#!/bin/sh\nif [ \"$1\" = workspace ] && [ \"$2\" = show ]; then echo development; exit 0; fi\nexit 1\n",
			),
			0o700,
		),
	)
	t.Setenv("PATH", bin)
	name, err := currentWorkspace(context.Background(), t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "development", name)
	_, err = os.Stat(marker)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
