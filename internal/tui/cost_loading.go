package tui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// LoadingState tracks the progress of plugin queries.
// It provides visual feedback while plugins are being queried asynchronously.
type LoadingState struct {
	spinner spinner.Model
	message string
}

// NewLoadingState creates a new loading state with spinner.
func NewLoadingState() *LoadingState {
	return &LoadingState{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(ColorSpinner)),
		),
		message: "Querying cost data from plugins...",
	}
}

// Init initializes the loading state (starts spinner).
func (l *LoadingState) Init() tea.Cmd {
	return l.spinner.Tick
}

// Update updates the loading state (spinner).
func (l *LoadingState) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.spinner, cmd = l.spinner.Update(msg)
	return cmd
}
