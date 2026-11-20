package cli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/conformance"
)

func pluginDevSkillDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "finfocus-plugin-dev")
}

func readPluginDevSkill(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(pluginDevSkillDir(t), rel))
	require.NoError(t, err)
	return string(data)
}

// TestFinfocusPluginDevSkillNamingCitesLiveChecks keeps the naming guidance
// tied to the conformance suite: every check ID it names must exist, and the
// flag it tells authors to pass must be registered.
func TestFinfocusPluginDevSkillNamingCitesLiveChecks(t *testing.T) {
	t.Parallel()

	docs := readPluginDevSkill(t, "SKILL.md") + readPluginDevSkill(t, "references/naming.md")
	require.Contains(t, docs, "## Naming conformance")

	suite, err := conformance.NewSuite(conformance.SuiteConfig{PluginPath: "finfocus-plugin-example"})
	require.NoError(t, err)
	live := map[string]bool{}
	for _, tc := range suite.GetTestCases() {
		live[tc.Name] = true
	}
	cited := regexp.MustCompile("`(Name_[A-Za-z]+)`").FindAllStringSubmatch(docs, -1)
	require.NotEmpty(t, cited)
	for _, match := range cited {
		assert.True(t, live[match[1]], "%s is cited by the skill but is not a conformance check", match[1])
	}
	for _, id := range []string{"Name_MatchesInstallName", "Name_MatchesPluginInfo", "Name_IsCanonicalForm"} {
		assert.Contains(t, docs, "`"+id+"`")
	}

	assert.Contains(t, docs, "--expected-name")
	assert.NotNil(t, NewPluginConformanceCmd().Flags().Lookup("expected-name"))
}

func TestFinfocusPluginDevSkillArchive(t *testing.T) {
	t.Parallel()

	dir := pluginDevSkillDir(t)
	archive, err := zip.OpenReader(filepath.Join(dir, "finfocus-plugin-dev.skill"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, archive.Close()) })

	packedFiles := map[string]bool{}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(file.Name, "finfocus-plugin-dev/")
		packedFiles[rel] = true
		packed, openErr := file.Open()
		require.NoError(t, openErr)
		got, readErr := io.ReadAll(packed)
		require.NoError(t, packed.Close())
		require.NoError(t, readErr)
		want, wantErr := os.ReadFile(filepath.Join(dir, rel))
		require.NoError(t, wantErr)
		assert.Equal(t, string(want), string(got), "%s is stale in the archive", file.Name)
	}

	sources := []string{"SKILL.md"}
	refs, err := os.ReadDir(filepath.Join(dir, "references"))
	require.NoError(t, err)
	for _, ref := range refs {
		sources = append(sources, "references/"+ref.Name())
	}
	for _, src := range sources {
		assert.True(t, packedFiles[src], "%s is missing from the archive", src)
	}
	assert.Len(t, packedFiles, len(sources), "the archive holds exactly SKILL.md and references/")
}
