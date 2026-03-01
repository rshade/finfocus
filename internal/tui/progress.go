package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Progress bar constants.
const (
	// DefaultProgressBarWidth is the default width of progress bars in characters.
	DefaultProgressBarWidth = 30
	// ProgressThresholdWarning is the percentage at which progress bars show warning color.
	ProgressThresholdWarning = 80
	// ProgressThresholdCritical is the percentage at which progress bars show critical color.
	ProgressThresholdCritical = 100
)

// ProgressBar renders a text-based progress bar.
type ProgressBar struct {
	Width   int
	Filled  string
	Empty   string
	ShowPct bool
}

// DefaultProgressBar returns a ProgressBar configured with the package defaults: Width set to DefaultProgressBarWidth, Filled set to "█", Empty set to "░", and ShowPct enabled.
func DefaultProgressBar() ProgressBar {
	return ProgressBar{
		Width:   DefaultProgressBarWidth,
		Filled:  "█",
		Empty:   "░",
		ShowPct: true,
	}
}

// Render returns a styled progress bar string.
func (p ProgressBar) Render(percent float64) string {
	if percent < 0 {
		percent = 0
	}
	if percent > ProgressThresholdCritical {
		percent = ProgressThresholdCritical
	}

	// Guard against negative or zero width to prevent panic.
	if p.Width <= 0 {
		if p.ShowPct {
			return fmt.Sprintf("%.0f%%", percent)
		}
		return ""
	}

	filled := int(percent / ProgressThresholdCritical * float64(p.Width))
	bar := strings.Repeat(p.Filled, filled) +
		strings.Repeat(p.Empty, p.Width-filled)

	// Apply color based on percentage.
	var style lipgloss.Style
	switch {
	case percent >= ProgressThresholdCritical:
		style = lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	case percent >= ProgressThresholdWarning:
		style = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	default:
		style = lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	}

	result := style.Render(bar)
	if p.ShowPct {
		result += fmt.Sprintf(" %.0f%%", percent)
	}
	return result
}
