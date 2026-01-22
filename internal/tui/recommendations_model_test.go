package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestNewRecommendationsSummary(t *testing.T) {
	t.Run("empty recommendations", func(t *testing.T) {
		summary := NewRecommendationsSummary(nil)

		assert.Equal(t, 0, summary.TotalCount)
		assert.Equal(t, 0.0, summary.TotalSavings)
		assert.Empty(t, summary.CountByAction)
		assert.Empty(t, summary.SavingsByAction)
		assert.Empty(t, summary.TopRecommendations)
	})

	t.Run("single recommendation", func(t *testing.T) {
		recs := []engine.Recommendation{
			{
				ResourceID:       "aws:ec2:Instance/i-123",
				Type:             "RIGHTSIZE",
				Description:      "Downsize instance",
				EstimatedSavings: 50.00,
				Currency:         "USD",
			},
		}

		summary := NewRecommendationsSummary(recs)

		assert.Equal(t, 1, summary.TotalCount)
		assert.Equal(t, 50.0, summary.TotalSavings)
		assert.Equal(t, "USD", summary.Currency)
		assert.Equal(t, 1, summary.CountByAction["RIGHTSIZE"])
		assert.Equal(t, 50.0, summary.SavingsByAction["RIGHTSIZE"])
		assert.Len(t, summary.TopRecommendations, 1)
	})

	t.Run("multiple recommendations with same action type", func(t *testing.T) {
		recs := []engine.Recommendation{
			{Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
			{Type: "RIGHTSIZE", EstimatedSavings: 30.00, Currency: "USD"},
			{Type: "RIGHTSIZE", EstimatedSavings: 20.00, Currency: "USD"},
		}

		summary := NewRecommendationsSummary(recs)

		assert.Equal(t, 3, summary.TotalCount)
		assert.Equal(t, 100.0, summary.TotalSavings)
		assert.Equal(t, 3, summary.CountByAction["RIGHTSIZE"])
		assert.Equal(t, 100.0, summary.SavingsByAction["RIGHTSIZE"])
	})

	t.Run("multiple action types", func(t *testing.T) {
		recs := []engine.Recommendation{
			{Type: "RIGHTSIZE", EstimatedSavings: 50.00, Currency: "USD"},
			{Type: "TERMINATE", EstimatedSavings: 100.00, Currency: "USD"},
			{Type: "RIGHTSIZE", EstimatedSavings: 30.00, Currency: "USD"},
			{Type: "DELETE_UNUSED", EstimatedSavings: 25.00, Currency: "USD"},
		}

		summary := NewRecommendationsSummary(recs)

		assert.Equal(t, 4, summary.TotalCount)
		assert.Equal(t, 205.0, summary.TotalSavings)
		assert.Equal(t, 2, summary.CountByAction["RIGHTSIZE"])
		assert.Equal(t, 1, summary.CountByAction["TERMINATE"])
		assert.Equal(t, 1, summary.CountByAction["DELETE_UNUSED"])
		assert.Equal(t, 80.0, summary.SavingsByAction["RIGHTSIZE"])
		assert.Equal(t, 100.0, summary.SavingsByAction["TERMINATE"])
	})

	t.Run("top 5 sorted by savings", func(t *testing.T) {
		recs := []engine.Recommendation{
			{ResourceID: "r1", EstimatedSavings: 10.00, Currency: "USD"},
			{ResourceID: "r2", EstimatedSavings: 50.00, Currency: "USD"},
			{ResourceID: "r3", EstimatedSavings: 30.00, Currency: "USD"},
			{ResourceID: "r4", EstimatedSavings: 80.00, Currency: "USD"},
			{ResourceID: "r5", EstimatedSavings: 20.00, Currency: "USD"},
			{ResourceID: "r6", EstimatedSavings: 100.00, Currency: "USD"},
			{ResourceID: "r7", EstimatedSavings: 40.00, Currency: "USD"},
		}

		summary := NewRecommendationsSummary(recs)

		require.Len(t, summary.TopRecommendations, 5)
		// Should be sorted descending: r6(100), r4(80), r2(50), r7(40), r3(30)
		assert.Equal(t, "r6", summary.TopRecommendations[0].ResourceID)
		assert.Equal(t, "r4", summary.TopRecommendations[1].ResourceID)
		assert.Equal(t, "r2", summary.TopRecommendations[2].ResourceID)
		assert.Equal(t, "r7", summary.TopRecommendations[3].ResourceID)
		assert.Equal(t, "r3", summary.TopRecommendations[4].ResourceID)
	})

	t.Run("fewer than 5 recommendations", func(t *testing.T) {
		recs := []engine.Recommendation{
			{ResourceID: "r1", EstimatedSavings: 50.00, Currency: "USD"},
			{ResourceID: "r2", EstimatedSavings: 30.00, Currency: "USD"},
		}

		summary := NewRecommendationsSummary(recs)

		require.Len(t, summary.TopRecommendations, 2)
		assert.Equal(t, "r1", summary.TopRecommendations[0].ResourceID)
		assert.Equal(t, "r2", summary.TopRecommendations[1].ResourceID)
	})
}

func TestRecommendationSortField(t *testing.T) {
	t.Run("sort field values", func(t *testing.T) {
		// Verify enum values are distinct
		assert.NotEqual(t, SortBySavings, SortByResourceID)
		assert.NotEqual(t, SortBySavings, SortByActionType)
		assert.NotEqual(t, SortByResourceID, SortByActionType)
	})

	t.Run("numRecommendationSortFields is correct", func(t *testing.T) {
		assert.Equal(t, 3, numRecommendationSortFields)
	})
}

// ============================================================================
// Phase 7: RecommendationsViewModel Tests (T039-T043)
// ============================================================================

// T039: Test RecommendationsViewModel state transitions.
func TestRecommendationsViewModel_StateTransitions(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 50.00, Currency: "USD"},
	}

	t.Run("initial state is list", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		assert.Equal(t, ViewStateList, model.state)
	})

	t.Run("has summary populated", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		require.NotNil(t, model.summary)
		assert.Equal(t, 2, model.summary.TotalCount)
		assert.Equal(t, 150.0, model.summary.TotalSavings)
	})

	t.Run("loading state with fetcher", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return recs, nil
		})
		assert.Equal(t, ViewStateLoading, model.state)
		assert.NotNil(t, model.loading)
	})
}

// ============================================================================
// Phase 8: Loading Spinner Tests (T060-T061)
// ============================================================================

// T060: Test loading state transitions in RecommendationsViewModel.
func TestRecommendationsViewModel_LoadingState(t *testing.T) {
	t.Run("loading state initialization", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return []engine.Recommendation{{ResourceID: "r1"}}, nil
		})
		assert.Equal(t, ViewStateLoading, model.state)
		assert.NotNil(t, model.loading)
		assert.NotNil(t, model.fetchCmd)
	})

	t.Run("Init returns spinner tick and fetch commands", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return nil, nil
		})
		cmd := model.Init()
		assert.NotNil(t, cmd)
	})

	t.Run("loading complete transitions to list state", func(t *testing.T) {
		recs := []engine.Recommendation{
			{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
			{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 50.00, Currency: "USD"},
		}

		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return recs, nil
		})
		assert.Equal(t, ViewStateLoading, model.state)

		// Simulate loading complete message
		loadingMsg := recommendationsLoadingMsg{recommendations: recs, err: nil}
		updatedModel, _ := model.Update(loadingMsg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateList, m.state)
		assert.Len(t, m.recommendations, 2)
		require.NotNil(t, m.summary)
		assert.Equal(t, 2, m.summary.TotalCount)
		assert.Equal(t, 150.0, m.summary.TotalSavings)
	})

	t.Run("loading error transitions to error state", func(t *testing.T) {
		expectedErr := assert.AnError

		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return nil, expectedErr
		})

		// Simulate loading error message
		loadingMsg := recommendationsLoadingMsg{recommendations: nil, err: expectedErr}
		updatedModel, _ := model.Update(loadingMsg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateError, m.state)
		assert.Equal(t, expectedErr, m.err)
	})

	t.Run("empty recommendations loading completes successfully", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return []engine.Recommendation{}, nil
		})

		loadingMsg := recommendationsLoadingMsg{recommendations: []engine.Recommendation{}, err: nil}
		updatedModel, _ := model.Update(loadingMsg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateList, m.state)
		assert.Empty(t, m.recommendations)
		require.NotNil(t, m.summary)
		assert.Equal(t, 0, m.summary.TotalCount)
	})
}

// T061: Test loading view rendering.
func TestRecommendationsViewModel_LoadingView(t *testing.T) {
	t.Run("loading state renders loading spinner", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return nil, nil
		})

		output := model.View()
		// RenderLoading includes the spinner and message
		assert.Contains(t, output, "Querying cost data from plugins...")
	})

	t.Run("error state renders error message", func(t *testing.T) {
		ctx := context.Background()
		model := NewRecommendationsViewModelWithLoading(ctx, func(_ context.Context) ([]engine.Recommendation, error) {
			return nil, assert.AnError
		})

		// Simulate error
		loadingMsg := recommendationsLoadingMsg{recommendations: nil, err: assert.AnError}
		updatedModel, _ := model.Update(loadingMsg)
		m := updatedModel.(*RecommendationsViewModel)

		output := m.View()
		assert.Contains(t, output, "Error:")
	})

	t.Run("quitting state renders empty", func(t *testing.T) {
		model := NewRecommendationsViewModel([]engine.Recommendation{})
		model.state = ViewStateQuitting

		output := model.View()
		assert.Empty(t, output)
	})
}

// T040: Test keyboard handlers.
func TestRecommendationsViewModel_KeyHandlers(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
	}

	t.Run("s key cycles sort", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		initialSort := model.sortBy

		model.cycleSort()

		assert.NotEqual(t, initialSort, model.sortBy)
	})

	t.Run("sort cycles through all fields", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)

		// Cycle through all sort fields
		for i := range numRecommendationSortFields {
			expected := (SortBySavings + RecommendationSortField(i)) % numRecommendationSortFields
			assert.Equal(t, expected, model.sortBy)
			model.cycleSort()
		}

		// Should be back to initial
		assert.Equal(t, SortBySavings, model.sortBy)
	})
}

// T041: Test filter logic.
func TestRecommendationsViewModel_FilterLogic(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "aws-ec2-1", Type: "RIGHTSIZE", Description: "Downsize instance", EstimatedSavings: 100.00},
		{ResourceID: "gcp-vm-2", Type: "TERMINATE", Description: "Remove unused", EstimatedSavings: 50.00},
		{ResourceID: "aws-rds-3", Type: "RIGHTSIZE", Description: "Scale down database", EstimatedSavings: 75.00},
	}

	t.Run("filter by resource ID", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.textInput.SetValue("aws")
		model.applyFilter()

		assert.Len(t, model.recommendations, 2) // aws-ec2-1 and aws-rds-3
	})

	t.Run("filter by action type", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.textInput.SetValue("TERMINATE")
		model.applyFilter()

		assert.Len(t, model.recommendations, 1)
		assert.Equal(t, "gcp-vm-2", model.recommendations[0].ResourceID)
	})

	t.Run("filter by description", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.textInput.SetValue("database")
		model.applyFilter()

		assert.Len(t, model.recommendations, 1)
		assert.Equal(t, "aws-rds-3", model.recommendations[0].ResourceID)
	})

	t.Run("clear filter restores all", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.textInput.SetValue("aws")
		model.applyFilter()
		assert.Len(t, model.recommendations, 2)

		model.textInput.SetValue("")
		model.applyFilter()
		assert.Len(t, model.recommendations, 3)
	})

	t.Run("filter updates summary", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		assert.Equal(t, 225.0, model.summary.TotalSavings) // 100 + 50 + 75

		model.textInput.SetValue("aws")
		model.applyFilter()

		assert.Equal(t, 175.0, model.summary.TotalSavings) // 100 + 75 (aws resources only)
	})
}

// T042: Test table rendering.
func TestRecommendationsTable(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", Description: "Downsize", EstimatedSavings: 100.00},
		{ResourceID: "r2", Type: "TERMINATE", Description: "Remove unused", EstimatedSavings: 50.00},
	}

	t.Run("creates list with correct rows", func(t *testing.T) {
		output := NewRecommendationsTable(recs, 10)

		// Output should contain recommendation data
		assert.Contains(t, output, "r1")
		assert.Contains(t, output, "RIGHTSIZE")
		assert.Contains(t, output, "r2")
		assert.Contains(t, output, "TERMINATE")
	})

	t.Run("empty recommendations creates empty list", func(t *testing.T) {
		output := NewRecommendationsTable([]engine.Recommendation{}, 10)

		// Empty list should be empty string
		assert.Empty(t, output)
	})
}

// T043: Test detail view rendering.
func TestRenderRecommendationDetail(t *testing.T) {
	rec := engine.Recommendation{
		ResourceID:       "aws:ec2:Instance/i-0abc123",
		Type:             "RIGHTSIZE",
		Description:      "Consider downsizing from m5.xlarge to m5.large",
		EstimatedSavings: 87.60,
		Currency:         "USD",
	}

	t.Run("renders all fields", func(t *testing.T) {
		output := RenderRecommendationDetail(rec, 80)

		assert.Contains(t, output, "aws:ec2:Instance/i-0abc123")
		assert.Contains(t, output, "RIGHTSIZE")
		assert.Contains(t, output, "87.60")
		assert.Contains(t, output, "USD")
		assert.Contains(t, output, "downsizing")
	})

	t.Run("shows navigation hints", func(t *testing.T) {
		output := RenderRecommendationDetail(rec, 80)

		assert.Contains(t, output, "Esc")
		assert.Contains(t, output, "Quit")
	})
}

// Test summary TUI rendering.
func TestRenderRecommendationsSummaryTUI(t *testing.T) {
	t.Run("renders nil summary gracefully", func(t *testing.T) {
		output := RenderRecommendationsSummaryTUI(nil, 80)
		assert.Contains(t, output, "No recommendations")
	})

	t.Run("renders summary with action types", func(t *testing.T) {
		summary := &RecommendationsSummary{
			TotalCount:      5,
			TotalSavings:    250.00,
			Currency:        "USD",
			CountByAction:   map[string]int{"RIGHTSIZE": 3, "TERMINATE": 2},
			SavingsByAction: map[string]float64{"RIGHTSIZE": 150.00, "TERMINATE": 100.00},
		}

		output := RenderRecommendationsSummaryTUI(summary, 80)

		assert.Contains(t, output, "5 recommendations")
		assert.Contains(t, output, "250.00")
		assert.Contains(t, output, "RIGHTSIZE")
		assert.Contains(t, output, "TERMINATE")
	})
}

// ============================================================================
// Additional Coverage Tests (T067-T069)
// ============================================================================

// Test Update method with keyboard messages.
func TestRecommendationsViewModel_UpdateKeyboard(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00, Currency: "USD"},
		{ResourceID: "r2", Type: "TERMINATE", EstimatedSavings: 50.00, Currency: "USD"},
	}

	t.Run("Enter key in list selects detail view", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		assert.Equal(t, ViewStateList, model.state)

		// Simulate Enter key
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateDetail, m.state)
		// Selection is managed by VirtualListModel internally
	})

	t.Run("Escape from detail returns to list", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.state = ViewStateDetail
		// Selection is managed by VirtualListModel

		// Simulate Escape key
		msg := tea.KeyMsg{Type: tea.KeyEscape}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateList, m.state)
	})

	t.Run("q key in list quits", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)

		// Simulate q key
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
		updatedModel, cmd := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateQuitting, m.state)
		// cmd should be tea.Quit
		assert.NotNil(t, cmd)
	})

	t.Run("Ctrl+C quits from list", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)

		// Simulate Ctrl+C
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}
		updatedModel, cmd := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateQuitting, m.state)
		assert.NotNil(t, cmd)
	})

	t.Run("Ctrl+C quits from detail", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.state = ViewStateDetail

		// Simulate Ctrl+C
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}
		updatedModel, cmd := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, ViewStateQuitting, m.state)
		assert.NotNil(t, cmd)
	})

	t.Run("slash key activates filter mode", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		assert.False(t, model.showFilter)

		// Simulate / key
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.True(t, m.showFilter)
	})

	t.Run("s key cycles sort", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		initialSort := model.sortBy

		// Simulate s key
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.NotEqual(t, initialSort, m.sortBy)
	})

	t.Run("escape clears active filter", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.textInput.SetValue("test")
		model.applyFilter()

		// Simulate Escape key to clear filter
		msg := tea.KeyMsg{Type: tea.KeyEscape}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, "", m.textInput.Value())
	})
}

// Test window resize handling.
func TestRecommendationsViewModel_WindowResize(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", EstimatedSavings: 100.00},
	}

	t.Run("handles window size message", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)

		// Simulate window resize
		msg := tea.WindowSizeMsg{Width: 120, Height: 40}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		assert.Equal(t, 120, m.width)
		assert.Equal(t, 40, m.height)
	})
}

// Test View method rendering for different states.
func TestRecommendationsViewModel_ViewStates(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE", Description: "Test", EstimatedSavings: 100.00, Currency: "USD"},
	}

	t.Run("renders detail view correctly", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.state = ViewStateDetail
		if model.virtualList != nil {
			model.virtualList.SetSelected(0)
		}

		output := model.View()
		assert.Contains(t, output, "r1")
		assert.Contains(t, output, "RIGHTSIZE")
	})

	t.Run("renders detail view bounds check", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.state = ViewStateDetail
		if model.virtualList != nil {
			model.virtualList.SetSelected(999) // Out of bounds (will be capped by VirtualListModel)
		}

		output := model.View()
		// VirtualListModel caps selection to valid range, so we'll still see valid data
		assert.Contains(t, output, "r1")
	})
}

// Test sorting applies correctly.
func TestRecommendationsViewModel_SortingVariants(t *testing.T) {
	recs := []engine.Recommendation{
		{ResourceID: "b-resource", Type: "TERMINATE", EstimatedSavings: 50.00},
		{ResourceID: "a-resource", Type: "RIGHTSIZE", EstimatedSavings: 100.00},
		{ResourceID: "c-resource", Type: "DELETE_UNUSED", EstimatedSavings: 25.00},
	}

	t.Run("sorts by resource ID", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.sortBy = SortByResourceID
		model.applySort()

		assert.Equal(t, "a-resource", model.recommendations[0].ResourceID)
		assert.Equal(t, "b-resource", model.recommendations[1].ResourceID)
		assert.Equal(t, "c-resource", model.recommendations[2].ResourceID)
	})

	t.Run("sorts by action type", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.sortBy = SortByActionType
		model.applySort()

		// DELETE_UNUSED < RIGHTSIZE < TERMINATE alphabetically
		assert.Equal(t, "DELETE_UNUSED", model.recommendations[0].Type)
		assert.Equal(t, "RIGHTSIZE", model.recommendations[1].Type)
		assert.Equal(t, "TERMINATE", model.recommendations[2].Type)
	})

	t.Run("sorts by savings (default)", func(t *testing.T) {
		model := NewRecommendationsViewModel(recs)
		model.sortBy = SortBySavings
		model.applySort()

		// Highest savings first
		assert.Equal(t, 100.0, model.recommendations[0].EstimatedSavings)
		assert.Equal(t, 50.0, model.recommendations[1].EstimatedSavings)
		assert.Equal(t, 25.0, model.recommendations[2].EstimatedSavings)
	})
}

// Test empty recommendations handling.
func TestRecommendationsViewModel_EmptyRecommendations(t *testing.T) {
	t.Run("enter on empty list does nothing", func(t *testing.T) {
		model := NewRecommendationsViewModel([]engine.Recommendation{})

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := model.Update(msg)
		m := updatedModel.(*RecommendationsViewModel)

		// Should stay in list state
		assert.Equal(t, ViewStateList, m.state)
	})

	t.Run("renders list view with filter active", func(t *testing.T) {
		model := NewRecommendationsViewModel([]engine.Recommendation{
			{ResourceID: "r1", Type: "RIGHTSIZE"},
		})
		model.showFilter = true

		output := model.View()
		assert.Contains(t, output, "Filter:")
	})
}

// Test SetVerbose function.
func TestRecommendationsViewModel_SetVerbose(t *testing.T) {
	model := NewRecommendationsViewModel([]engine.Recommendation{})

	model.SetVerbose(true)
	assert.True(t, model.verbose)

	model.SetVerbose(false)
	assert.False(t, model.verbose)
}
