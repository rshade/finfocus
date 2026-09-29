package cli_test

import (
	"testing"

	"github.com/rshade/finfocus/internal/config"
)

// isolateFromPulumiProject changes the working directory to a temp dir so
// tests are not influenced by a Pulumi.yaml in the repository tree. t.Chdir
// restores the original directory on test completion.
func isolateFromPulumiProject(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
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
