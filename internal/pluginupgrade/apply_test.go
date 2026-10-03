package pluginupgrade_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

func planFor(t *testing.T, dir, target string) (*pluginupgrade.Project, *pluginupgrade.Plan) {
	t.Helper()
	project, err := pluginupgrade.Detect(dir)
	require.NoError(t, err)
	plan, err := pluginupgrade.NewPlan(project, target, latest)
	require.NoError(t, err)
	return project, plan
}

func TestApplyScaffoldedPlugin(t *testing.T) {
	t.Parallel()

	dir := scaffoldedFixture(t)
	untouched := map[string]string{}
	for _, rel := range []string{
		"vendor/example.com/dep/dep.go", "internal/openapi/spec.go",
		"internal/pricing/compat_test.go", "internal/pricing/info.go",
	} {
		untouched[rel] = readFile(t, dir, rel)
	}
	project, plan := planFor(t, dir, latest)

	changed, err := pluginupgrade.Apply(project, plan)
	require.NoError(t, err)
	assert.Equal(t, []string{"go.mod", "internal/pricing/calculator.go"}, changed)

	assert.Equal(t, `module github.com/example/finfocus-plugin-scaffolded

go 1.27.1

require (
	github.com/rshade/finfocus-spec v0.7.0
	github.com/rs/zerolog v1.35.1
)
`, readFile(t, dir, "go.mod"))

	assert.Equal(t, `package pricing

import (
	"context"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	// PluginVersion is the semantic version of this plugin.
	PluginVersion = "0.1.0"
	// SpecVersion is the finfocus-spec protocol version this plugin was compiled against.
	SpecVersion = "v0.7.0"
)

const builtAgainst = "finfocus-spec 0.6.1"

// GetPluginInfo returns metadata about this plugin.
func GetPluginInfo(context.Context) *pbc.GetPluginInfoResponse {
	return &pbc.GetPluginInfoResponse{Version: PluginVersion, SpecVersion: SpecVersion}
}
`, readFile(t, dir, "internal/pricing/calculator.go"),
		"only the SpecVersion literal changes, gaining the v prefix; other strings stay")

	for rel, before := range untouched {
		assert.Equal(t, before, readFile(t, dir, rel), rel)
	}

	again, err := pluginupgrade.Detect(dir)
	require.NoError(t, err)
	assert.Equal(t, latest, again.Version)
}

func TestApplyRaisesGoDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		target string
		wantGo string
	}{
		{target: latest, wantGo: "1.27.1"},
		{target: "v0.6.0", wantGo: "1.25.7"},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			dir := copyFixture(t, "finfocus-v0.5.3")
			project, plan := planFor(t, dir, tc.target)

			changed, err := pluginupgrade.Apply(project, plan)
			require.NoError(t, err)
			assert.Equal(t, []string{"go.mod", "internal/plugin/plugin.go"}, changed)

			assert.Equal(t, `module github.com/example/finfocus-plugin-modern

go `+tc.wantGo+`

require github.com/rshade/finfocus-spec `+tc.target+` // pinned for the DryRun API

replace github.com/rshade/finfocus-spec => ../finfocus-spec
`, readFile(t, dir, "go.mod"), "comment and replace survive")
			assert.Contains(t, readFile(t, dir, "internal/plugin/plugin.go"), "\tSpecVersion = \""+tc.target+"\"\n")
		})
	}
}

func TestApplyUpToDateChangesNothing(t *testing.T) {
	t.Parallel()

	dir := copyFixture(t, "finfocus-v0.7.0")
	before := readFile(t, dir, "go.mod")
	project, plan := planFor(t, dir, latest)
	require.True(t, plan.UpToDate)

	changed, err := pluginupgrade.Apply(project, plan)
	require.NoError(t, err)
	assert.Empty(t, changed)
	assert.Equal(t, before, readFile(t, dir, "go.mod"))
}

func TestApplyRefusesChangedSource(t *testing.T) {
	t.Parallel()

	dir := copyFixture(t, "finfocus-v0.5.3")
	project, plan := planFor(t, dir, latest)
	require.NoError(t,
		os.WriteFile(filepath.Join(dir, "internal", "plugin", "plugin.go"), []byte("package plugin\n"), 0o600))

	_, err := pluginupgrade.Apply(project, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal/plugin/plugin.go changed since it was scanned")
	assert.Contains(t, readFile(t, dir, "go.mod"), "v0.5.3", "go.mod is written last, after every source edit succeeds")
}

func TestApplyRefusesChangedGoMod(t *testing.T) {
	t.Parallel()

	dir := copyFixture(t, "finfocus-v0.5.3")
	project, plan := planFor(t, dir, latest)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module changed\n"), 0o600))
	pluginBefore := readFile(t, dir, "internal/plugin/plugin.go")

	_, err := pluginupgrade.Apply(project, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod changed since it was scanned")
	assert.Equal(t, pluginBefore, readFile(t, dir, "internal/plugin/plugin.go"), "nothing is written")
}

func TestApplyReportsPartialWrite(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can write read-only files")
	}

	dir := copyFixture(t, "finfocus-v0.5.3")
	project, plan := planFor(t, dir, latest)
	goModBefore := readFile(t, dir, "go.mod")
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	changed, err := pluginupgrade.Apply(project, plan)
	require.Error(t, err)
	assert.Equal(t, goModBefore, readFile(t, dir, "go.mod"), "a failed write leaves the original intact")
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".upgrade-", "no temporary file is left behind")
	}
	assert.Equal(t, []string{"internal/plugin/plugin.go"}, changed)
	assert.Contains(t, err.Error(), "writing go.mod")
	assert.Contains(t, err.Error(), "already updated: internal/plugin/plugin.go")
}
