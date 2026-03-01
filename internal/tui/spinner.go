package tui

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
)

// DefaultSpinner returns a spinner.Model configured with the Dot spinner and styled using ColorInfo.
func DefaultSpinner() spinner.Model {
	return spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(ColorInfo)),
	)
}
