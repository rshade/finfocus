package pluginupgrade_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

func TestDetectScaffoldedPlugin(t *testing.T) {
	t.Parallel()

	p, err := pluginupgrade.Detect(scaffoldedFixture(t))
	require.NoError(t, err)

	assert.Equal(t, "v0.6.1", p.Version)
	assert.Equal(t, "1.27.1", p.GoVersion)
	assert.Empty(t, p.ReplacePath)
	assert.Equal(t,
		[]pluginupgrade.SpecVersionDecl{{File: "internal/pricing/calculator.go", Value: "0.6.1"}},
		p.SpecVersionDecls,
		"skips vendor/, _test.go files, files that do not import finfocus-spec, and non-version values")
}

func TestDetectReplacedPlugin(t *testing.T) {
	t.Parallel()

	p, err := pluginupgrade.Detect(filepath.Join("testdata", "finfocus-v0.5.3"))
	require.NoError(t, err)

	assert.Equal(t, "v0.5.3", p.Version)
	assert.Equal(t, "../finfocus-spec", p.ReplacePath)
	assert.Equal(t,
		[]pluginupgrade.SpecVersionDecl{{File: "internal/plugin/plugin.go", Value: "v0.5.2"}},
		p.SpecVersionDecls)
}

func TestDetectVersionedReplace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gomod := "module example.com/p\n\ngo 1.27.1\n\nrequire github.com/rshade/finfocus-spec v0.6.2\n\n" +
		"replace github.com/rshade/finfocus-spec v0.6.2 => ../finfocus-spec\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600))

	p, err := pluginupgrade.Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "../finfocus-spec", p.ReplacePath)
	assert.Equal(t, "v0.6.2", p.ReplaceVersion)
}

func TestDetectPseudoVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gomod := "module example.com/p\n\ngo 1.27.1\n\n" +
		"require github.com/rshade/finfocus-spec v0.6.2-0.20260920000000-abcdefabcdef\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600))

	p, err := pluginupgrade.Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, "v0.6.2-0.20260920000000-abcdefabcdef", p.Version)
	assert.Empty(t, p.SpecVersionDecls)
}

func TestDetectNotAPlugin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "no spec dependency", dir: filepath.Join("testdata", "no-spec")},
		{name: "no go.mod", dir: t.TempDir()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := pluginupgrade.Detect(tc.dir)
			require.ErrorIs(t, err, pluginupgrade.ErrNotPlugin)
		})
	}
}

func TestDetectInvalidGoMod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module\nrequire (\n"), 0o600))

	_, err := pluginupgrade.Detect(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod")
}

func TestDetectInvalidGoSource(t *testing.T) {
	t.Parallel()

	dir := copyFixture(t, "finfocus-v0.7.0")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package main\nfunc {"), 0o600))

	_, err := pluginupgrade.Detect(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken.go")
}
