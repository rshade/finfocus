package cli

import (
	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
)

// dryRunRequested reports whether the global --dry-run flag is set, from the ax
// context or, for commands run without ax.Execute, from the flag itself.
func dryRunRequested(cmd *cobra.Command) bool {
	if cmd != nil && ax.DryRunFromContext(commandContext(cmd)) {
		return true
	}
	if cmd == nil || cmd.Flags().Lookup("dry-run") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("dry-run")
	return err == nil && value
}
