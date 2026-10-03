package pluginupgrade_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// copyFixture copies testdata/<name> into a fresh temporary directory, so a
// test can apply edits without touching the checked-in fixture.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	})
	require.NoError(t, err)
	return dst
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(data)
}

// scaffoldedFixture copies the plugin-init-shaped fixture and adds a vendored
// file declaring SpecVersion. It is created here because .gitignore excludes
// vendor/ directories, so it cannot be checked in under testdata.
func scaffoldedFixture(t *testing.T) string {
	t.Helper()
	dir := copyFixture(t, "finfocus-v0.6.1")
	vendored := filepath.Join(dir, "vendor", "example.com", "dep")
	require.NoError(t, os.MkdirAll(vendored, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(vendored, "dep.go"), []byte(`package dep

import _ "github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

// SpecVersion belongs to a vendored dependency and must not be rewritten.
const SpecVersion = "0.6.1"
`), 0o600))
	return dir
}
