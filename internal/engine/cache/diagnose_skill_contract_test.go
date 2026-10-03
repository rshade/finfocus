package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	berrors "go.etcd.io/bbolt/errors"
)

func TestDiagnoseSkillQuotesCacheFile(t *testing.T) {
	t.Parallel()

	docs := readDiagnoseSkillDocs(t)
	assert.Contains(t, docs, dbFileName)
	assert.Contains(t, docs, ErrInvalidTTL.Error())
	assert.True(t, isCorruptionError(berrors.ErrInvalid))
	assert.True(t, isCorruptionError(berrors.ErrChecksum))
	assert.True(t, isCorruptionError(berrors.ErrVersionMismatch))
	assert.False(t, isCorruptionError(berrors.ErrTimeout))
	for _, name := range []string{"ErrInvalid", "ErrChecksum", "ErrVersionMismatch", "ErrTimeout"} {
		assert.Contains(t, docs, name)
	}
}

func readDiagnoseSkillDocs(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "agent-skills", "finfocus-diagnose")
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	return b.String()
}
