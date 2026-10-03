package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/pluginskill"
)

// skillInstaller installs the plugin agent skills into a plugin project.
type skillInstaller interface {
	Install(ctx context.Context, dir, version string) pluginskill.Result
}

// pluginSkills is replaced in tests so they never run npx.
var pluginSkills skillInstaller = pluginskill.New() //nolint:gochecknoglobals // test seam, set once in TestMain

// printSkillResult reports a skill install on the command's output. A warning
// never fails the command; it comes with the command to run later.
func printSkillResult(cmd *cobra.Command, res pluginskill.Result) {
	if len(res.Paths) > 0 {
		cmd.Println("Installed FinFocus agent skills:")
		for _, p := range res.Paths {
			cmd.Printf("  %s\n", p)
		}
	}
	if res.Warning == "" {
		return
	}
	cmd.Printf("⚠️  %s\n", strings.ReplaceAll(res.Warning, "\n", "\n    "))
	if !res.Installed {
		cmd.Printf("To add the agent skills later, run in the plugin directory:\n  %s\n", res.Command)
	}
}
