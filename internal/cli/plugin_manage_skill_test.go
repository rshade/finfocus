package cli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

func pluginManageSkillDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "plugin-manage")
}

func readPluginManageSkill(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(pluginManageSkillDir(t), rel))
	require.NoError(t, err)
	return string(data)
}

func TestPluginManageSkillDocumentsLiveFlags(t *testing.T) {
	t.Parallel()

	docs := readPluginManageSkill(t, "SKILL.md")
	commands := map[string]*cobra.Command{
		"install":     NewPluginInstallCmd(),
		"update":      NewPluginUpdateCmd(),
		"remove":      NewPluginRemoveCmd(),
		"list":        NewPluginListCmd(),
		"validate":    NewPluginValidateCmd(),
		"inspect":     NewPluginInspectCmd(),
		"conformance": NewPluginConformanceCmd(),
		"certify":     NewPluginCertifyCmd(),
	}
	flags := map[string][]string{
		"install": {
			"force", "no-save", "clean", "plugin-dir", "metadata",
			"skip-checksum", "fallback-to-latest", "no-fallback",
		},
		"update":      {"version", "plugin-dir", "skip-checksum"},
		"remove":      {"keep-config", "plugin-dir"},
		"list":        {"available", "verbose", "output"},
		"validate":    {"plugin"},
		"inspect":     {"version", "json"},
		"conformance": {"mode", "category", "output", "output-file", "filter", "timeout"},
		"certify":     {"mode", "output", "timeout"},
	}
	for name, cmd := range commands {
		for _, flag := range flags[name] {
			require.NotNil(t, cmd.Flags().Lookup(flag), "plugin %s has --%s", name, flag)
			assert.Contains(t, docs, "--"+flag, "SKILL.md documents plugin %s --%s", name, flag)
		}
		assert.Contains(t, docs, "finfocus plugin "+name)
	}

	assert.Equal(t, []string{"uninstall", "rm"}, NewPluginRemoveCmd().Aliases)
	assert.Contains(t, docs, "aliases: uninstall, rm")
	assert.Contains(t, docs, "mutually exclusive")
	assert.Contains(t, docs, "finfocus config routes test")
	assert.Contains(t, docs, "FINFOCUS_STRICT_COMPATIBILITY")
}

func TestPluginManageSkillListsEmbeddedRegistry(t *testing.T) {
	t.Parallel()

	entries, err := registry.GetAllPluginEntries()
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	layout := readPluginManageSkill(t, filepath.Join("references", "registry-and-layout.md"))
	for _, entry := range entries {
		assert.Contains(t, layout, "| `"+entry.Name+"` |", "registry plugin %s is listed", entry.Name)
		assert.Contains(t, layout, entry.Repository)
	}
}

func TestPluginManageSkillTroubleshootingQuotesLiveErrors(t *testing.T) {
	t.Parallel()

	docs := readPluginManageSkill(t, filepath.Join("references", "troubleshooting.md"))
	assert.Contains(t, docs, "is currently being modified by another process (lock file")
	assert.Contains(t, docs, "GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits")
	assert.Contains(t, docs, "integrity check failed")
	assert.Contains(t, docs, "already installed. Use --force to reinstall")
}

func TestPluginManageSkillArchive(t *testing.T) {
	t.Parallel()

	dir := pluginManageSkillDir(t)
	archive, err := zip.OpenReader(filepath.Join(dir, "plugin-manage.skill"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, archive.Close())
	})

	names := map[string]bool{}
	for _, file := range archive.File {
		names[file.Name] = true
		rel := strings.TrimPrefix(file.Name, "plugin-manage/")
		packed, openErr := file.Open()
		require.NoError(t, openErr)
		got, readErr := io.ReadAll(packed)
		require.NoError(t, packed.Close())
		require.NoError(t, readErr)
		want, wantErr := os.ReadFile(filepath.Join(dir, rel))
		require.NoError(t, wantErr)
		assert.Equal(t, string(want), string(got), "%s is stale in the archive", file.Name)
	}
	assert.True(t, names["plugin-manage/SKILL.md"])
	assert.True(t, names["plugin-manage/references/registry-and-layout.md"])
	assert.True(t, names["plugin-manage/references/troubleshooting.md"])
}
