package weblog_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVendoredViewerFixturesMatch keeps the zerolog-viewer copies identical to
// testdata. The package tests read only those copies, so they still pass when
// the finfocus tree is absent.
func TestVendoredViewerFixturesMatch(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sample.ndjson", "filter_cases.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			canonical := filepath.Join("testdata", name)
			vendored := filepath.Join("..", "..", "packages", "zerolog-viewer", "test", "fixtures", name)
			want, err := os.ReadFile(canonical)
			require.NoError(t, err)
			got, err := os.ReadFile(vendored)
			require.NoError(t, err)
			require.NotEmpty(t, want)
			assert.Equal(t, string(want), string(got))
		})
	}
}
