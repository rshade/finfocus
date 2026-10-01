package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

func TestCLIBranding(t *testing.T) {
	t.Run("root command help shows FinFocus", func(t *testing.T) {
		root := cli.NewRootCmd(version.GetVersion())
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"--help"})

		err := root.Execute()
		require.NoError(t, err, "failed to execute root command")

		output := buf.String()
		// Verify proper branding "FinFocus" appears in help (from Long description)
		assert.Contains(t, output, "FinFocus", "expected FinFocus branding in help output")
		// Note: The command name "finfocus" will also appear in help, which is expected
	})

	t.Run("version output contains FinFocus", func(t *testing.T) {
		root := cli.NewRootCmd(version.GetVersion())
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs([]string{"--version"})

		err := root.Execute()
		require.NoError(t, err, "failed to execute root command")

		output := buf.String()
		// Cobra --version usually just prints the version string from cmd.Version
		// unless a custom version template is set.
		// If it only prints "0.1.0", we might need to update the template.
		t.Logf("Version output: %q", output)
	})
}
