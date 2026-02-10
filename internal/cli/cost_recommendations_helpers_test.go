package cli_test

import "github.com/spf13/cobra"

// findSubcommand finds a subcommand by name in a cobra.Command.
// Shared helper used across dismiss, history, and undismiss tests.
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, sub := range cmd.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	return nil
}
