package cli_test

import (
	"context"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

const unsupportedXML = "unsupported output format: xml"

// outputFormatCases lists every command whose --output selects a format, with
// the arguments it needs to reach that check and the message it reports.
func outputFormatCases() []struct {
	args    []string
	message string
} {
	return []struct {
		args    []string
		message string
	}{
		{[]string{"cost", "forecast"}, unsupportedXML},
		{[]string{"cost", "projected"}, unsupportedXML},
		{[]string{"cost", "actual"}, unsupportedXML},
		{[]string{"cost", "estimate"}, unsupportedXML},
		{[]string{"cost", "cluster"}, unsupportedXML},
		{[]string{"cost", "anomalies"}, unsupportedXML},
		{[]string{"cost", "recommendations"}, unsupportedXML},
		{[]string{"cost", "recommendations", "history", "rec-1"}, unsupportedXML},
		{[]string{"cost", "history", "collect-daily"}, unsupportedXML},
		{[]string{"cost", "history", "view"}, unsupportedXML},
		{[]string{"cost", "history", "diff"}, unsupportedXML},
		{[]string{"overview"}, unsupportedXML},
		{[]string{"plugin", "list"}, unsupportedXML},
		{[]string{"plugin", "upgrade"}, unsupportedXML},
		{[]string{"plugin", "conformance", "."}, "invalid output format"},
		{[]string{"config", "routes", "list"}, unsupportedXML},
		{[]string{"config", "routes", "test", "aws:ec2/instance:Instance"}, unsupportedXML},
		{[]string{"config", "validate"}, "invalid --output"},
		{[]string{"analyzer", "check"}, unsupportedXML},
		{[]string{"carbon", "actual", "--start", "2026-01-01", "--end", "2026-03-01"}, "flag --output"},
	}
}

// outputIsNotAFormat names commands whose --output flag is a file path.
func outputIsNotAFormat() map[string]bool {
	return map[string]bool{"finfocus plugin certify": true}
}

// An --output value a command cannot render is a user-input error (exit 2),
// rejected before any state or plugin is loaded.
func TestUnsupportedOutputExitsValidation(t *testing.T) {
	// Not parallel: t.Setenv and t.Chdir keep the run away from the real home
	// and any Pulumi project.
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Chdir(t.TempDir())

	for _, tc := range outputFormatCases() {
		result := axtest.Run(context.Background(), t, cli.NewRootCmd("test-version"),
			append(tc.args, "--output", "xml"))
		assert.Equal(t, ax.ExitValidation, result.ExitCode, "%v: %s", tc.args, result.Stderr)
		assert.Contains(t, string(result.Stderr), tc.message, "%v", tc.args)
	}
}

// Every command with --output is either in outputFormatCases or named in
// outputIsNotAFormat, so a new command cannot skip the exit-2 check.
//
//nolint:paralleltest // NewRootCmd sets the process-wide resolved project directory
func TestUnsupportedOutputCoversEveryCommand(t *testing.T) {
	root := cli.NewRootCmd("test-version")
	covered := outputIsNotAFormat()
	for _, tc := range outputFormatCases() {
		cmd, _, err := root.Find(tc.args)
		require.NoError(t, err, "%v", tc.args)
		covered[cmd.CommandPath()] = true
	}

	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.Runnable() && cmd.LocalFlags().Lookup("output") != nil {
			assert.True(t, covered[cmd.CommandPath()], "%s has --output but is not in outputFormatCases",
				cmd.CommandPath())
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
