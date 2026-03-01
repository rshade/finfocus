//go:build integration
// +build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// viewContains is a test helper that asserts the model's View() output
// contains the expected substring.
func viewContains(t *testing.T, model tui.OverviewModel, substr string) {
	t.Helper()
	view := model.View().Content
	assert.Contains(t, view, substr,
		"expected View() to contain %q, got:\n%s", substr, truncateView(view))
}

// viewNotContains is a test helper that asserts the model's View() output
// does NOT contain the expected substring.
func viewNotContains(t *testing.T, model tui.OverviewModel, substr string) {
	t.Helper()
	view := model.View().Content
	assert.NotContains(t, view, substr,
		"expected View() NOT to contain %q, got:\n%s", substr, truncateView(view))
}

// truncateView truncates long view strings for test failure messages.
func truncateView(s string) string {
	const maxLen = 300
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}
	return s
}

// updateModel sends a message to the model and returns the updated model.
// It performs the type assertion from tea.Model back to tui.OverviewModel.
func updateModel(t *testing.T, model tui.OverviewModel, msg tea.Msg) (tui.OverviewModel, tea.Cmd) {
	t.Helper()
	updated, cmd := model.Update(msg)
	m, ok := updated.(tui.OverviewModel)
	require.True(t, ok, "Update() should return tui.OverviewModel, got %T", updated)
	return m, cmd
}

// isInInitializingState checks if the model's View() output indicates
// the Initializing state (contains phase checklist content).
func isInInitializingState(model tui.OverviewModel) bool {
	view := model.View().Content
	// Initializing view contains the phase checklist items
	return strings.Contains(view, "Loading stack state") ||
		strings.Contains(view, "Detecting changes") ||
		strings.Contains(view, "Merging resources")
}

// isInLoadingState checks if the model's View() output indicates the Loading state.
func isInLoadingState(model tui.OverviewModel) bool {
	view := model.View().Content
	return strings.Contains(view, "Loading...")
}

// isInListState checks if the model's View() output indicates the List state.
func isInListState(model tui.OverviewModel) bool {
	view := model.View().Content
	return strings.Contains(view, "Sort:")
}

// isInDetailState checks if the model's View() output indicates the Detail state.
func isInDetailState(model tui.OverviewModel) bool {
	view := model.View().Content
	return strings.Contains(view, "RESOURCE DETAIL")
}

// isInErrorState checks if the model's View() output indicates the Error state.
func isInErrorState(model tui.OverviewModel) bool {
	view := model.View().Content
	return strings.HasPrefix(view, "Error:")
}

// isInQuittingState checks if the model's View() output indicates the Quitting state.
func isInQuittingState(model tui.OverviewModel) bool {
	return model.View().Content == ""
}

// newTestRows creates a slice of OverviewRow for testing with the given count.
func newTestRows(count int) []engine.OverviewRow {
	types := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
	}
	rows := make([]engine.OverviewRow, count)
	for i := range rows {
		rows[i] = engine.OverviewRow{
			URN:    "urn:pulumi:stack::project::" + types[i%len(types)] + "::resource-" + strings.Repeat("x", i),
			Type:   types[i%len(types)],
			Status: engine.StatusActive,
		}
	}
	return rows
}

// TestTUI_PhaseProgression verifies that the OverviewModel transitions through
// ViewStateInitializing → ViewStateLoading → ViewStateList as phase messages,
// data-ready messages, and all-resources-loaded messages are received.
func TestTUI_PhaseProgression(t *testing.T) {
	ctx := context.Background()

	// Create model in Initializing state (nil skeleton rows)
	model, initCmd := tui.NewOverviewModel(ctx, nil, 0, nil, nil)
	require.NotNil(t, initCmd, "Init should return a spinner command")

	// Verify initial state is Initializing
	assert.True(t, isInInitializingState(model),
		"model should start in Initializing state")

	// Send phase messages 0-5 and verify the model updates the phase checklist
	phases := tui.GetPhaseNames()
	for i, phase := range phases {
		phaseMsg := tui.OverviewPhaseMsg{Phase: phase, Index: i}
		var cmd tea.Cmd
		model, cmd = updateModel(t, model, phaseMsg)
		_ = cmd

		// Model should remain in Initializing state after phase messages
		assert.True(t, isInInitializingState(model),
			"model should remain in Initializing state after phase %d (%s)", i, phase)
	}

	// Verify the View() output contains the last phase name
	viewContains(t, model, phases[len(phases)-1])

	// Send OverviewDataReadyMsg to transition Initializing → Loading
	testRows := newTestRows(3)
	dataMsg := tui.OverviewDataReadyMsg{
		Rows:       testRows,
		TotalCount: 3,
		StackName:  "test-stack",
	}
	model, _ = updateModel(t, model, dataMsg)

	assert.True(t, isInLoadingState(model),
		"model should transition to Loading state after OverviewDataReadyMsg")

	// Send OverviewAllResourcesLoadedMsg to transition Loading → List
	allLoadedMsg := tui.OverviewAllResourcesLoadedMsg{}
	model, _ = updateModel(t, model, allLoadedMsg)

	assert.True(t, isInListState(model),
		"model should transition to List state after OverviewAllResourcesLoadedMsg")

	// Verify the list view contains expected elements
	viewContains(t, model, "Sort:")

	t.Run("UnexpectedMessageOrder", func(t *testing.T) {
		// Send OverviewDataReadyMsg while still in ViewStateInitializing
		// (before any phase messages) — should not panic and should handle gracefully.
		freshModel, _ := tui.NewOverviewModel(ctx, nil, 0, nil, nil)
		assert.True(t, isInInitializingState(freshModel),
			"fresh model should start in Initializing state")

		// Send DataReadyMsg directly (skipping all phase messages)
		earlyDataMsg := tui.OverviewDataReadyMsg{
			Rows:       newTestRows(2),
			TotalCount: 2,
			StackName:  "early-stack",
		}

		// This must not panic
		updatedModel, cmd := freshModel.Update(earlyDataMsg)
		require.NotNil(t, updatedModel, "Update should return non-nil model")
		_ = cmd

		m, ok := updatedModel.(tui.OverviewModel)
		require.True(t, ok, "Update should return tui.OverviewModel")

		// After receiving DataReadyMsg, model should be in Loading state
		assert.True(t, isInLoadingState(m),
			"model should transition to Loading state even without prior phase messages")
	})
}

// TestTUI_KeyboardNavigation verifies that keyboard events correctly
// navigate between List → Detail → List states and that 'q' triggers quit.
func TestTUI_KeyboardNavigation(t *testing.T) {
	ctx := context.Background()

	// Create model with skeleton rows and bring to List state
	testRows := []engine.OverviewRow{
		{
			URN:    "urn:pulumi:stack::project::aws:ec2/instance:Instance::instance-1",
			Type:   "aws:ec2/instance:Instance",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 100.0,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:pulumi:stack::project::aws:s3/bucket:Bucket::bucket-1",
			Type:   "aws:s3/bucket:Bucket",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 5.0,
				Currency:    "USD",
			},
		},
		{
			URN:    "urn:pulumi:stack::project::aws:rds/instance:Instance::db-1",
			Type:   "aws:rds/instance:Instance",
			Status: engine.StatusActive,
			ProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 200.0,
				Currency:    "USD",
			},
		},
	}

	model, _ := tui.NewOverviewModel(ctx, testRows, 3, nil, nil)

	// Transition to List state via OverviewAllResourcesLoadedMsg
	model, _ = updateModel(t, model, tui.OverviewAllResourcesLoadedMsg{})
	require.True(t, isInListState(model), "model should be in List state")

	// Test: Down arrow key moves cursor (model stays in List state)
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}
	model, _ = updateModel(t, model, downMsg)
	assert.True(t, isInListState(model), "model should remain in List state after Down key")

	// Test: Enter key transitions to Detail state
	enterMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	model, _ = updateModel(t, model, enterMsg)
	assert.True(t, isInDetailState(model), "model should transition to Detail state after Enter")
	viewContains(t, model, "RESOURCE DETAIL")
	viewContains(t, model, "Press ESC to return")

	// Test: Escape key returns to List state
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	model, _ = updateModel(t, model, escMsg)
	assert.True(t, isInListState(model), "model should return to List state after Escape")
	viewNotContains(t, model, "RESOURCE DETAIL")

	// Test: 'q' key triggers quit
	qMsg := tea.KeyPressMsg{Text: "q"}
	model, cmd := updateModel(t, model, qMsg)
	assert.True(t, isInQuittingState(model), "model should transition to Quitting after 'q'")
	assert.NotNil(t, cmd, "'q' key should return a tea.Quit command")
}

// TestTUI_ErrorStateOnInitFailure verifies that sending OverviewInitErrorMsg
// transitions the model to ViewStateError and that View() displays the error.
func TestTUI_ErrorStateOnInitFailure(t *testing.T) {
	ctx := context.Background()

	// Create model in Initializing state
	model, _ := tui.NewOverviewModel(ctx, nil, 0, nil, nil)
	assert.True(t, isInInitializingState(model),
		"model should start in Initializing state")

	// Send error message
	testErr := errors.New("test init error: no Pulumi project found")
	errMsg := tui.OverviewInitErrorMsg{Err: testErr}
	model, cmd := updateModel(t, model, errMsg)

	// Verify transition to Error state
	assert.True(t, isInErrorState(model),
		"model should transition to Error state after OverviewInitErrorMsg")

	// Verify the View() output contains the error message text
	viewContains(t, model, "test init error")
	viewContains(t, model, "no Pulumi project found")

	// Verify Err() returns the error
	require.Error(t, model.Err())
	assert.Contains(t, model.Err().Error(), "test init error")

	// Verify tea.Quit was returned
	assert.NotNil(t, cmd, "OverviewInitErrorMsg should return a tea.Quit command")

	t.Run("ErrorFromLoadingState", func(t *testing.T) {
		// Test that error can also occur from Loading state
		loadingModel, _ := tui.NewOverviewModel(ctx, newTestRows(2), 2, nil, nil)
		assert.True(t, isInLoadingState(loadingModel),
			"model with skeleton rows should start in Loading state")

		loadErr := errors.New("plugin connection failed")
		loadingModel, loadCmd := updateModel(t, loadingModel, tui.OverviewInitErrorMsg{Err: loadErr})

		assert.True(t, isInErrorState(loadingModel),
			"model should transition to Error from Loading state")
		viewContains(t, loadingModel, "plugin connection failed")
		require.Error(t, loadingModel.Err())
		assert.NotNil(t, loadCmd)
	})

	t.Run("EmptyErrorMessage", func(t *testing.T) {
		// Test with minimal error message
		emptyModel, _ := tui.NewOverviewModel(ctx, nil, 0, nil, nil)
		minErr := errors.New("")
		emptyModel, emptyCmd := updateModel(t, emptyModel, tui.OverviewInitErrorMsg{Err: minErr})

		assert.True(t, isInErrorState(emptyModel),
			"model should transition to Error even with empty error message")
		require.Error(t, emptyModel.Err())
		assert.NotNil(t, emptyCmd)
	})
}

// TestTUI_WindowResizeNoPanic verifies that sending tea.WindowSizeMsg to
// the model in various states (Initializing, Loading, List) does not cause
// a panic and returns a non-nil model.
func TestTUI_WindowResizeNoPanic(t *testing.T) {
	ctx := context.Background()
	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}

	t.Run("Initializing", func(t *testing.T) {
		model, _ := tui.NewOverviewModel(ctx, nil, 0, nil, nil)
		require.True(t, isInInitializingState(model), "model should start in Initializing state")

		model, cmd := updateModel(t, model, resizeMsg)
		assert.True(t, isInInitializingState(model),
			"model should remain in Initializing state after WindowSizeMsg")
		assert.Nil(t, cmd, "WindowSizeMsg should return nil command")
	})

	t.Run("Loading", func(t *testing.T) {
		model, _ := tui.NewOverviewModel(ctx, newTestRows(3), 3, nil, nil)
		require.True(t, isInLoadingState(model), "model should start in Loading state")

		model, cmd := updateModel(t, model, resizeMsg)
		assert.True(t, isInLoadingState(model),
			"model should remain in Loading state after WindowSizeMsg")
		assert.Nil(t, cmd, "WindowSizeMsg should return nil command")
	})

	t.Run("List", func(t *testing.T) {
		model, _ := tui.NewOverviewModel(ctx, newTestRows(5), 5, nil, nil)
		model, _ = updateModel(t, model, tui.OverviewAllResourcesLoadedMsg{})
		require.True(t, isInListState(model), "model should be in List state")

		model, cmd := updateModel(t, model, resizeMsg)
		assert.True(t, isInListState(model),
			"model should remain in List state after WindowSizeMsg")
		assert.Nil(t, cmd, "WindowSizeMsg should return nil command")
	})

	t.Run("SmallWindow", func(t *testing.T) {
		// Test with very small window dimensions (edge case)
		smallResize := tea.WindowSizeMsg{Width: 20, Height: 5}
		model, _ := tui.NewOverviewModel(ctx, newTestRows(3), 3, nil, nil)
		model, _ = updateModel(t, model, tui.OverviewAllResourcesLoadedMsg{})
		require.True(t, isInListState(model), "model should be in List state")

		model, cmd := updateModel(t, model, smallResize)
		assert.True(t, isInListState(model),
			"model should remain in List state after small WindowSizeMsg")
		assert.Nil(t, cmd, "WindowSizeMsg should return nil command")
	})
}
