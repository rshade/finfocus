package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/pluginskill"
)

// skillCalls records fake plugin skill installs by absolute project directory,
// so parallel tests can check their own directory without interfering.
var skillCalls sync.Map //nolint:gochecknoglobals // shared fake for the package

func TestMain(m *testing.M) {
	cli.SetPluginSkillsForTest(func(_ context.Context, dir, version string) pluginskill.Result {
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		skillCalls.Store(abs, version)
		return pluginskill.Result{
			Installed: true,
			Source:    pluginskill.Source(version),
			Command:   pluginskill.Command(version),
			Paths:     []string{".agents/skills/finfocus-plugin-dev", "skills-lock.json"},
		}
	})
	os.Exit(m.Run())
}

// skillInstalledIn reports whether the fake installer ran for dir.
func skillInstalledIn(t *testing.T, dir string) bool {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs %s: %v", dir, err)
	}
	_, ok := skillCalls.Load(abs)
	return ok
}
