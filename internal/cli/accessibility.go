package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// addAccessibilityFlags adds accessibility flags that the command does not
// already define. Overview keeps its --plain, --no-color, and --force-color
// flags. cost history view keeps its own --plain, so these flags are local
// to each output command and are not registered on the cost parent.
func addAccessibilityFlags(cmd *cobra.Command) {
	if cmd.Flags().Lookup("no-color") == nil {
		cmd.Flags().Bool("no-color", false, "Disable colored output and interactive TUI")
	}
	if cmd.Flags().Lookup("plain") == nil {
		cmd.Flags().Bool("plain", false, "Plain text without colors, borders, or an interactive TUI")
	}
	if cmd.Flags().Lookup("color") == nil {
		cmd.Flags().Bool("color", false, "Force colored output when stdout is not a terminal")
	}
	if cmd.Flags().Lookup("high-contrast") == nil {
		cmd.Flags().Bool("high-contrast", false, "Use brighter high-contrast colors")
	}
}

// scopeAccessibilityFlagsToBudget rewrites the help of the accessibility flags
// for a command whose result table is always plain text. The cost diff table of
// `cost projected` has no color or interactive mode, so these flags only change
// the budget box printed after it. The flags stay accepted so scripts that
// pass them keep working.
func scopeAccessibilityFlagsToBudget(cmd *cobra.Command) {
	usage := map[string]string{
		"no-color":      "Disable colored budget output (the cost diff table is always plain text)",
		"plain":         "Plain budget output without colors or borders (the cost diff table is always plain text)",
		"color":         "Force a colored budget box when stdout is not a terminal (the cost diff table is always plain text)",
		"high-contrast": "Use brighter colors in the budget box (the cost diff table is always plain text)",
	}
	for name, text := range usage {
		if flag := cmd.Flags().Lookup(name); flag != nil {
			flag.Usage = text
		}
	}
}

func flagBool(cmd *cobra.Command, name string) (bool, bool) {
	if cmd == nil {
		return false, false
	}
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return false, false
	}
	value, err := cmd.Flags().GetBool(name)
	if err != nil {
		return false, false
	}
	return value, flag.Changed
}

func accessibilityFromCmd(cmd *cobra.Command) tui.Accessibility {
	noColor, noColorSet := flagBool(cmd, "no-color")
	plain, plainSet := flagBool(cmd, "plain")
	colorOn, colorSet := flagBool(cmd, "color")
	forceOn, forceSet := flagBool(cmd, "force-color")
	high, highSet := flagBool(cmd, "high-contrast")
	return tui.ResolveAccessibility(
		tui.Accessibility{
			NoColor:      noColor,
			Plain:        plain,
			ForceColor:   colorOn || forceOn,
			HighContrast: high,
		},
		tui.Accessibility{
			NoColor:      noColorSet,
			Plain:        plainSet,
			ForceColor:   colorSet || forceSet,
			HighContrast: highSet,
		},
	)
}

func outputModeFromCmd(cmd *cobra.Command) tui.OutputMode {
	access := accessibilityFromCmd(cmd)
	writer := io.Writer(os.Stdout)
	if cmd != nil {
		writer = cmd.OutOrStdout()
	}
	return tui.DetectResolvedOutputMode(writer, access)
}

func applyOverviewAccessibility(cmd *cobra.Command, params overviewParams) overviewParams {
	access := accessibilityFromCmd(cmd)
	params.plain = access.Plain
	params.noColor = access.NoColor
	params.forceColor = access.ForceColor
	return params
}

// renderBudgetForAccess renders the budget box. Plain text wins over color.
// A non-terminal writer stays plain unless color was forced. High contrast
// recolors the styled box and leaves the default palette untouched.
func renderBudgetForAccess(w io.Writer, status *engine.BudgetStatus, access tui.Accessibility) error {
	if status == nil {
		return nil
	}
	if access.Plain || access.NoColor || (!isWriterTerminal(w) && !access.ForceColor) {
		return renderPlainBudget(w, status)
	}
	return renderStyledBudget(w, status, access.HighContrast)
}
