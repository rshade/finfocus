package cli_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/cli"
)

type upgradeOutput struct {
	Plan struct {
		Current    string `json:"current"`
		Target     string `json:"target"`
		UpToDate   bool   `json:"up_to_date"`
		RequiredGo string `json:"required_go"`
		Hops       []struct {
			To       string `json:"to"`
			Guide    string `json:"guide"`
			GuideURL string `json:"guide_url"`
		} `json:"hops"`
	} `json:"plan"`
	DryRun    bool     `json:"dry_run"`
	Changed   []string `json:"changed_files"`
	NextSteps []string `json:"next_steps"`
}

// copyUpgradeFixture copies a pluginupgrade fixture into a temporary directory.
func copyUpgradeFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("..", "pluginupgrade", "testdata", name)
	dst := t.TempDir()
	require.NoError(t, filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o750)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o600)
	}))
	return dst
}

func runUpgrade(t *testing.T, args ...string) axtest.Result {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	root := cli.NewRootCmd("test")
	return axtest.Run(context.Background(), t, root, append([]string{"plugin", "upgrade"}, args...))
}

//nolint:paralleltest // Builds a root command and sets FINFOCUS_HOME.
func TestPluginUpgradeDryRunJSON(t *testing.T) {
	dir := copyUpgradeFixture(t, "finfocus-v0.5.3")
	before, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)

	result := runUpgrade(t, "--dir", dir, "--dry-run", "--output", "json")
	require.Zero(t, result.ExitCode, string(result.Stderr))

	var out upgradeOutput
	require.NoError(t, json.Unmarshal(result.Stdout, &out), string(result.Stdout))
	assert.True(t, out.DryRun)
	assert.Equal(t, "v0.5.3", out.Plan.Current)
	assert.Equal(t, pluginsdk.SpecVersion, out.Plan.Target)
	assert.Equal(t, "1.27.1", out.Plan.RequiredGo)
	require.NotEmpty(t, out.Plan.Hops)
	assert.Equal(t, "v0.5.7", out.Plan.Hops[0].To)
	assert.Equal(t, "to-v0.5.7.md", out.Plan.Hops[0].Guide)
	assert.Contains(t, out.Plan.Hops[0].GuideURL, "agent-skills/finfocus-plugin-upgrade/references/to-v0.5.7.md")
	assert.Empty(t, out.Changed)

	after, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a dry run writes nothing")
}

//nolint:paralleltest // Builds a root command and sets FINFOCUS_HOME.
func TestPluginUpgradeDryRunTable(t *testing.T) {
	dir := copyUpgradeFixture(t, "finfocus-v0.5.3")

	result := runUpgrade(t, "--dir", dir, "--dry-run", "--to", "v0.6.0")
	require.Zero(t, result.ExitCode, string(result.Stderr))

	out := string(result.Stdout)
	assert.Contains(t, out, "github.com/rshade/finfocus-spec v0.5.3 (go 1.25.5)")
	assert.Contains(t, out, "Target:  v0.6.0")
	assert.Contains(t, out, "Go:      raise the go directive to 1.25.7")
	assert.Contains(t, out, "v0.5.7  DryRun handlers take a context")
	assert.Contains(t, out, "v0.6.0  Batch validation returns a result type")
	assert.NotContains(t, out, "v0.6.1")
	assert.Contains(t, out, "! internal/plugin/plugin.go declares SpecVersion \"v0.5.2\"")
	assert.Contains(t, out, "Dry run: no files changed.")
}

//nolint:paralleltest // Builds a root command and sets FINFOCUS_HOME.
func TestPluginUpgradeApply(t *testing.T) {
	dir := copyUpgradeFixture(t, "finfocus-v0.6.1")

	result := runUpgrade(t, "--dir", dir, "--allow-dirty")
	require.Zero(t, result.ExitCode, string(result.Stderr))

	out := string(result.Stdout)
	assert.Contains(t, out, `! internal/pricing/calculator.go declares SpecVersion "0.6.1" without the v prefix`)
	assert.Contains(t, out, "Changed files:\n  go.mod\n  internal/pricing/calculator.go\n")
	assert.Contains(t, out, "go mod tidy")

	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(gomod), "github.com/rshade/finfocus-spec "+pluginsdk.SpecVersion)
}

//nolint:paralleltest // Builds a root command and sets FINFOCUS_HOME.
func TestPluginUpgradeUpToDate(t *testing.T) {
	dir := t.TempDir()
	gomod := "module example.com/current\n\ngo 1.27.1\n\nrequire github.com/rshade/finfocus-spec " +
		pluginsdk.SpecVersion + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600))

	result := runUpgrade(t, "--dir", dir)
	require.Zero(t, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Plugin is up to date (finfocus-spec "+pluginsdk.SpecVersion+").")
	assert.NotContains(t, string(result.Stdout), "go mod tidy")
}

//nolint:paralleltest // Builds a root command and sets FINFOCUS_HOME.
func TestPluginUpgradeErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    func(t *testing.T) []string
		wantErr string
	}{
		{
			name:    "invalid output is rejected before reading the project",
			args:    func(t *testing.T) []string { return []string{"--dir", t.TempDir(), "--output", "yaml"} },
			wantErr: "unsupported output format: yaml",
		},
		{
			name: "not a plugin",
			args: func(t *testing.T) []string {
				return []string{"--dir", copyUpgradeFixture(t, "no-spec"), "--dry-run"}
			},
			wantErr: "not a FinFocus plugin",
		},
		{
			name: "target above core",
			args: func(t *testing.T) []string {
				return []string{"--dir", copyUpgradeFixture(t, "finfocus-v0.5.3"), "--dry-run", "--to", "v99.0.0"}
			},
			wantErr: "invalid target version",
		},
		{
			name: "apply refuses a directory outside git",
			args: func(t *testing.T) []string {
				return []string{"--dir", copyUpgradeFixture(t, "finfocus-v0.5.3")}
			},
			wantErr: "not a git repository",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := runUpgrade(t, tc.args(t)...)
			assert.NotZero(t, result.ExitCode)
			assert.Contains(t, string(result.Stderr), tc.wantErr)
		})
	}
}
