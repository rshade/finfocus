package registry

import (
	"os"
	"testing"

	"github.com/rshade/finfocus/internal/config"
)

func TestMain(m *testing.M) {
	// A shell that exports the variable must not change tests that assume the
	// default; tests of the mode set it themselves.
	_ = os.Unsetenv(config.EnvSecurePlugins)
	os.Exit(m.Run())
}
