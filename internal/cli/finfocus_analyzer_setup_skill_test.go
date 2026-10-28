package cli

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinfocusAnalyzerSetupSkillArchive(t *testing.T) {
	t.Parallel()

	dir := analyzerSetupSkillDir(t)
	skill, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skill), "pulumi-language-finfocus")

	archive, err := zip.OpenReader(filepath.Join(dir, "finfocus-analyzer-setup.skill"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, archive.Close())
	})

	wantNames := map[string]bool{
		"finfocus-analyzer-setup/SKILL.md":                      true,
		"finfocus-analyzer-setup/references/troubleshooting.md": true,
	}
	gotNames := map[string]bool{}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		gotNames[file.Name] = true
		rel := strings.TrimPrefix(file.Name, "finfocus-analyzer-setup/")
		packed, openErr := file.Open()
		require.NoError(t, openErr)
		got, readErr := io.ReadAll(packed)
		require.NoError(t, packed.Close())
		require.NoError(t, readErr)
		want, wantErr := os.ReadFile(filepath.Join(dir, rel))
		require.NoError(t, wantErr)
		assert.Equal(t, string(want), string(got), "%s is stale in the archive", file.Name)
	}
	assert.Equal(t, wantNames, gotNames)
}

func analyzerSetupSkillDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "finfocus-analyzer-setup")
}
