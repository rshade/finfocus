package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testGolden compares stripped (no ANSI) output against a golden file.
// Set UPDATE_GOLDEN=1 to regenerate.
func testGolden(t *testing.T, name string, actual string) {
	t.Helper()
	stripped := ansi.Strip(actual)
	golden := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, []byte(stripped), 0o644))
		return
	}
	expected, err := os.ReadFile(golden)
	require.NoError(t, err, "golden file %s missing; run UPDATE_GOLDEN=1", golden)
	assert.Equal(t, string(expected), stripped,
		"golden mismatch for %s; run UPDATE_GOLDEN=1 to update", name)
}
