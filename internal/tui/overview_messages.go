package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rshade/finfocus/internal/engine"
)

// OverviewPreviewStartedMsg is sent when the user presses 'p' to start loading
// pending changes. The TUI uses it to show the loading indicator.
type OverviewPreviewStartedMsg struct{}

// OverviewPreviewTickMsg is sent every second while preview is running.
// The TUI uses it to display an elapsed timer in the header.
type OverviewPreviewTickMsg struct {
	Elapsed time.Duration
}

// OverviewChangesReadyMsg is sent when pulumi preview completes.
// StatusByURN maps Pulumi URN strings to their ResourceStatus (creating/updating/etc.).
// Rows whose URN is absent retain StatusActive.
//
// If preview failed, StatusByURN is empty and HasChanges is false; the error
// is logged by the sender. The TUI remains usable in state-only mode.
type OverviewChangesReadyMsg struct {
	StatusByURN map[string]engine.ResourceStatus
	HasChanges  bool
	ChangeCount int
}

// OverviewSetStateOnlyMsg is sent after OverviewDataReadyMsg when state-only
// loading is active. It sets isStateOnly=true and provides the on-demand
// preview command for the 'p' key binding. This cannot be embedded in
// OverviewDataReadyMsg because previewCmd is only built after plugin startup,
// which happens after initial data loading.
type OverviewSetStateOnlyMsg struct {
	PreviewCmd tea.Cmd
}
