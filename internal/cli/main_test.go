package cli_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/pluginskill"
)

// skillCalls records fake plugin skill installs by absolute project directory,
// so parallel tests can check their own directory without interfering.
var skillCalls sync.Map //nolint:gochecknoglobals // shared fake for the package

func TestMain(m *testing.M) {
	if os.Getenv("FINFOCUS_LANGHOST_TEST_CHILD") == "ignore-sigint" {
		signal.Ignore(os.Interrupt)
		fmt.Fprintln(os.Stdout, "ready")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
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
