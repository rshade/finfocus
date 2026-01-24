package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

// TestCLI_TUI_OutputModes verifies that the CLI correctly selects between TUI and plain output.
func TestCLI_TUI_OutputModes(t *testing.T) {
	h := helpers.NewCLIHelper(t)
	// Use relative path from test/integration/cli
	planFile := filepath.Join("..", "..", "fixtures", "plans", "aws-simple-plan.json")

	t.Run("JSON output bypasses TUI", func(t *testing.T) {
		output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "json")
		require.NoError(t, err)
		require.Contains(t, output, `"summary"`)
		require.NotContains(t, output, "COST SUMMARY")
	})

	t.Run("NDJSON output bypasses TUI", func(t *testing.T) {
		output, err := h.Execute("cost", "projected", "--pulumi-json", planFile, "--output", "ndjson")
		require.NoError(t, err)
		require.NotContains(t, output, "COST SUMMARY")
	})

	t.Run("Plain table fallback when not a TTY", func(t *testing.T) {
		// In tests, stdout is captured via pipe, so it should fall back to plain table
		output := h.ExecuteOrFail("cost", "projected", "--pulumi-json", planFile)

		// Legacy plain text headers (from internal/engine/project.go)
		require.Contains(t, output, "COST SUMMARY")
		require.Contains(t, output, "============")
		require.Contains(t, output, "Total Monthly Cost")

		// Ensure no ANSI escape codes in plain output
		require.NotContains(t, output, "\x1b[")
	})

	t.Run("NO_COLOR respects standard", func(t *testing.T) {
		// Force NO_COLOR and verify output is plain
		t.Setenv("NO_COLOR", "1")

		output := h.ExecuteOrFail("cost", "projected", "--pulumi-json", planFile)
		require.NotContains(t, output, "\x1b[")
		require.Contains(t, output, "COST SUMMARY")
	})
}
