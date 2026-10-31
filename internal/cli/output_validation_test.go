package cli_test

import (
	"context"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/cli"
)

// An --output value a command cannot render is a user-input error (exit 2),
// rejected before any state or plugin is loaded.
func TestUnsupportedOutputExitsValidation(t *testing.T) {
	// Not parallel: t.Setenv and t.Chdir keep the run away from the real home
	// and any Pulumi project.
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Chdir(t.TempDir())

	commands := [][]string{
		{"cost", "forecast"},
		{"cost", "projected"},
		{"cost", "actual"},
		{"cost", "estimate"},
		{"cost", "cluster"},
		{"cost", "recommendations"},
		{"cost", "recommendations", "history", "rec-1"},
		{"cost", "history", "view"},
		{"cost", "history", "diff"},
		{"overview"},
		{"plugin", "list"},
		{"plugin", "upgrade"},
		{"config", "routes", "list"},
		{"analyzer", "check"},
	}
	for _, args := range commands {
		result := axtest.Run(context.Background(), t, cli.NewRootCmd("test-version"),
			append(args, "--output", "xml"))
		assert.Equal(t, ax.ExitValidation, result.ExitCode, "%v: %s", args, result.Stderr)
		assert.Contains(t, string(result.Stderr), "unsupported output format: xml", "%v", args)
	}
}
