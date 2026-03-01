package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/rshade/finfocus/internal/engine"
)

// OverviewPreviewTickMsg is sent every second while preview is running.
// The TUI uses it to trigger an elapsed-time update (computed from m.previewLoadStart).
type OverviewPreviewTickMsg struct{}

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
//
// DetectErrMsg is a short (≤60 chars) human-readable description of any error
// that occurred during change detection. When non-empty it is surfaced in the
// status bar so the user knows why state-only mode is active.
type OverviewSetStateOnlyMsg struct {
	PreviewCmd   tea.Cmd
	DetectErrMsg string
}

// BudgetDataReadyMsg delivers budget health data from the background fetch
// goroutine to the TUI model. It is sent once by the budget fetch goroutine
// in overviewInitAndEnrich(). The TUI stores the result and triggers a
// re-render of the list view footer and detail view budget section.
type BudgetDataReadyMsg struct {
	Result *engine.BudgetResult // Complete budget result (nil on failure)
	Error  error                // Fetch error (nil on success)
}
