package plugin_test

import (
	"os"
	"testing"

	"github.com/rshade/finfocus/internal/config"
)

func TestMain(m *testing.M) {
	// A shell that exports the variable must not change install tests that
	// assume the default; the mode has its own tests in internal/registry.
	_ = os.Unsetenv(config.EnvSecurePlugins)
	os.Exit(m.Run())
}
