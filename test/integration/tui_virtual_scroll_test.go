package integration_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// TestVirtualScrolling_LargeDataset tests the recommendations TUI with a large dataset
// to verify virtual scrolling performance and correctness.
func TestVirtualScrolling_LargeDataset(t *testing.T) {
	// Create a large dataset (1000 recommendations)
	recommendations := make([]engine.Recommendation, 1000)
	for i := range recommendations {
		recommendations[i] = engine.Recommendation{
			ResourceID:       "resource-" + string(rune('A'+i%26)) + string(rune('0'+(i/26)%10)),
			Type:             "RIGHTSIZE",
			Description:      "Test recommendation",
			EstimatedSavings: float64(i * 10),
			Currency:         "USD",
		}
	}

	model := tui.NewRecommendationsViewModel(recommendations)
	require.NotNil(t, model)

	// Initialize the model
	_ = model.Init()

	// Simulate window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	// Render the view (should only render visible portion)
	view := model.View()
	assert.NotEmpty(t, view)

	// View should contain the summary
	assert.Contains(t, view, "RECOMMENDATIONS SUMMARY")
	assert.Contains(t, view, "1000 recommendations")

	// View should contain table header
	assert.Contains(t, view, "Resource")
	assert.Contains(t, view, "Action")
	assert.Contains(t, view, "Savings")

	// View should NOT contain all 1000 items (virtual scrolling)
	// With height=40, summary=10, we expect ~30 rows visible
	// We can't count exact rows due to styling, but we can verify
	// that the view is reasonable in size (not all 1000 items)
	assert.Less(t, len(view), 50000, "View should not render all 1000 items")
}

// TestVirtualScrolling_NavigationKeys tests keyboard navigation with virtual scrolling.
func TestVirtualScrolling_NavigationKeys(t *testing.T) {
	// Create a medium dataset (100 recommendations)
	recommendations := make([]engine.Recommendation, 100)
	for i := range recommendations {
		recommendations[i] = engine.Recommendation{
			ResourceID:       "resource-" + string(rune('A'+i%26)),
			Type:             "RIGHTSIZE",
			Description:      "Test recommendation",
			EstimatedSavings: float64(i * 10),
			Currency:         "USD",
		}
	}

	model := tui.NewRecommendationsViewModel(recommendations)
	require.NotNil(t, model)

	// Initialize with window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	tests := []struct {
		name        string
		key         tea.KeyMsg
		description string
	}{
		{
			name:        "down arrow navigation",
			key:         tea.KeyMsg{Type: tea.KeyDown},
			description: "Should move selection down",
		},
		{
			name:        "up arrow navigation",
			key:         tea.KeyMsg{Type: tea.KeyUp},
			description: "Should move selection up",
		},
		{
			name:        "j key navigation",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
			description: "Should move selection down (vim)",
		},
		{
			name:        "k key navigation",
			key:         tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
			description: "Should move selection up (vim)",
		},
		{
			name:        "page down navigation",
			key:         tea.KeyMsg{Type: tea.KeyPgDown},
			description: "Should jump down one page",
		},
		{
			name:        "page up navigation",
			key:         tea.KeyMsg{Type: tea.KeyPgUp},
			description: "Should jump up one page",
		},
		{
			name:        "home key navigation",
			key:         tea.KeyMsg{Type: tea.KeyHome},
			description: "Should jump to start",
		},
		{
			name:        "end key navigation",
			key:         tea.KeyMsg{Type: tea.KeyEnd},
			description: "Should jump to end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Send the key message
			updatedModel, cmd := model.Update(tt.key)
			model = updatedModel.(*tui.RecommendationsViewModel)

			// Verify model still works (no panics)
			require.NotNil(t, model)

			// Verify view can be rendered
			view := model.View()
			assert.NotEmpty(t, view)

			// Cmd may be nil or a valid command
			_ = cmd
		})
	}
}

// TestVirtualScrolling_SortingAndFiltering tests that virtual scrolling works
// correctly when combined with sorting and filtering operations.
func TestVirtualScrolling_SortingAndFiltering(t *testing.T) {
	// Create a dataset with varied savings
	recommendations := []engine.Recommendation{
		{
			ResourceID:       "r-high",
			Type:             "TERMINATE",
			EstimatedSavings: 1000.0,
			Currency:         "USD",
			Description:      "High savings",
		},
		{
			ResourceID:       "r-medium",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 500.0,
			Currency:         "USD",
			Description:      "Medium savings",
		},
		{
			ResourceID:       "r-low",
			Type:             "DELETE_UNUSED",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Low savings",
		},
		{
			ResourceID:       "r-tiny",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 10.0,
			Currency:         "USD",
			Description:      "Tiny savings",
		},
	}

	model := tui.NewRecommendationsViewModel(recommendations)
	require.NotNil(t, model)

	// Initialize with window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	t.Run("sort cycle works with virtual list", func(t *testing.T) {
		// Press 's' to cycle sort
		sortMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
		updatedModel, _ := model.Update(sortMsg)
		model = updatedModel.(*tui.RecommendationsViewModel)

		// View should still render
		view := model.View()
		assert.NotEmpty(t, view)
		assert.Contains(t, view, "RECOMMENDATIONS SUMMARY")
	})

	t.Run("filter works with virtual list", func(t *testing.T) {
		// Press '/' to activate filter
		filterMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
		updatedModel, _ := model.Update(filterMsg)
		model = updatedModel.(*tui.RecommendationsViewModel)

		// View should show filter input
		view := model.View()
		assert.Contains(t, view, "Filter:")
	})
}

// TestVirtualScrolling_DetailView tests that detail view still works with virtual scrolling.
func TestVirtualScrolling_DetailView(t *testing.T) {
	recommendations := []engine.Recommendation{
		{
			ResourceID:       "test-resource",
			Type:             "RIGHTSIZE",
			EstimatedSavings: 100.0,
			Currency:         "USD",
			Description:      "Test detail",
		},
	}

	model := tui.NewRecommendationsViewModel(recommendations)
	require.NotNil(t, model)

	// Initialize with window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	// Press Enter to view details
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = model.Update(enterMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	// View should show detail page
	view := model.View()
	assert.Contains(t, view, "RECOMMENDATION DETAIL")
	assert.Contains(t, view, "test-resource")
	assert.Contains(t, view, "RIGHTSIZE")
}

// TestVirtualScrolling_EmptyList tests virtual scrolling with empty recommendations.
func TestVirtualScrolling_EmptyList(t *testing.T) {
	model := tui.NewRecommendationsViewModel([]engine.Recommendation{})
	require.NotNil(t, model)

	// Initialize with window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	// View should render empty state
	view := model.View()
	assert.Contains(t, view, "RECOMMENDATIONS SUMMARY")
	assert.Contains(t, view, "0 recommendations")
}

// TestVirtualScrolling_Performance tests that rendering performance is acceptable
// even with very large datasets (10,000+ items).
func TestVirtualScrolling_Performance(t *testing.T) {
	// Create a very large dataset (10,000 recommendations)
	recommendations := make([]engine.Recommendation, 10000)
	for i := range recommendations {
		recommendations[i] = engine.Recommendation{
			ResourceID:       "resource-" + string(rune('A'+i%26)) + string(rune('0'+(i/100)%10)),
			Type:             "RIGHTSIZE",
			Description:      "Performance test recommendation with longer description to simulate real data",
			EstimatedSavings: float64(i % 1000),
			Currency:         "USD",
		}
	}

	model := tui.NewRecommendationsViewModel(recommendations)
	require.NotNil(t, model)

	// Initialize with window size
	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(*tui.RecommendationsViewModel)

	// Render view multiple times to verify consistent performance
	for i := 0; i < 10; i++ {
		view := model.View()
		assert.NotEmpty(t, view)

		// Navigate down
		downMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(downMsg)
		model = updatedModel.(*tui.RecommendationsViewModel)
	}

	// Final view should still be valid
	finalView := model.View()
	assert.NotEmpty(t, finalView)
	assert.Contains(t, finalView, "RECOMMENDATIONS SUMMARY")
	assert.Contains(t, finalView, "10000 recommendations")
}
