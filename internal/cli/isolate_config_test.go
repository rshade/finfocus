package cli_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// isolateFromPulumiProject changes the working directory to a temp dir so
// tests are not influenced by a Pulumi.yaml in the repository tree. The
// original directory is restored via t.Cleanup.
func isolateFromPulumiProject(t *testing.T) {
	t.Helper()
	oldwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldwd)) })
}

// isolateConfig isolates the test from the user's real ~/.finfocus directory
// by pointing FINFOCUS_HOME to a temp dir and resetting the global config
// singleton. This prevents budget text from leaking into JSON output when
// the developer has budgets configured locally.
func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
}
