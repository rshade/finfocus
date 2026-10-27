package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// CLI tests isolate process environment and working directory.
func TestCostProjected_PulumiBaseline(t *testing.T) {
	for _, name := range []string{"default", "show-sames", "create-only"} {
		t.Run(name, func(t *testing.T) {
			fixture, err := filepath.Abs("testdata/pulumi/preview-" + name + ".json")
			require.NoError(t, err)
			golden, err := filepath.Abs(
				"testdata/pulumi/projected-baseline-" + name + ".golden.json",
			)
			require.NoError(t, err)
			isolateConfig(t)
			isolateFromPulumiProject(t)
			t.Setenv("FINFOCUS_LOG_LEVEL", "error")
			t.Setenv("FINFOCUS_CACHE_DIR", t.TempDir())
			result := axtest.Run(
				context.Background(),
				t,
				cli.NewRootCmd("test"),
				[]string{"cost", "projected", "--pulumi-json", fixture, "--output", "json"},
			)
			require.Equal(t, 0, result.ExitCode, "%s", result.Stderr)
			doc, _ := normalizeProjectedOutput(t, result.Stdout)
			delete(doc["finfocus"].(map[string]interface{}), "stack")
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			require.NoError(t, enc.Encode(doc))
			if os.Getenv("UPDATE_GOLDEN") != "" {
				require.NoError(t, os.WriteFile(golden, buf.Bytes(), 0o644))
				return
			}
			expected, err := os.ReadFile(golden)
			require.NoError(t, err)
			assert.JSONEq(t, string(expected), buf.String())
		})
	}
}
