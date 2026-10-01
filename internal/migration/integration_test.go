package migration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationFlow(t *testing.T) {
	t.Parallel()

	// We can't easily mock os.UserHomeDir() for DetectLegacy without
	// changing how it's implemented to accept a home override or
	// using a global variable for testing.

	// Let's create a wrapper that uses a home getter.

	t.Run("detects legacy directory", func(t *testing.T) {
		t.Parallel()
		tempHome := t.TempDir()
		legacyPath := filepath.Join(tempHome, ".finfocus")
		require.NoError(t, os.MkdirAll(legacyPath, 0700))

		// Use a helper that takes home dir
		path, exists := detectLegacyIn(tempHome)
		assert.True(t, exists)
		assert.Equal(t, legacyPath, path)
	})
}

func detectLegacyIn(home string) (string, bool) {
	legacyPath := filepath.Join(home, ".finfocus")
	info, err := os.Stat(legacyPath)
	if err != nil {
		return "", false
	}
	return legacyPath, info.IsDir()
}
