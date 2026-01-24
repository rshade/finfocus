package list_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui/list"
)

// TestVirtualListModel_ViewRendersOnlyVisibleRows tests that only visible rows are rendered.
func TestVirtualListModel_ViewRendersOnlyVisibleRows(t *testing.T) {
	// Create 1000 items
	items := make([]string, 1000)
	for i := range items {
		items[i] = "item"
	}

	renderFunc := func(item string, selected bool) string {
		if selected {
			return "> " + item
		}
		return "  " + item
	}

	// Viewport height of 20 means only ~20 rows should be rendered
	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	view := model.View()

	// Count rendered lines
	lines := strings.Split(view, "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// Should render approximately viewport height (20), not all 1000 items
	assert.LessOrEqual(t, nonEmptyLines, 25, "Should render ~20 rows, not all 1000")
	assert.Greater(t, nonEmptyLines, 0, "Should render at least some rows")
}

// TestVirtualListModel_ViewUpdatesWithScroll tests that view updates when scrolling.
func TestVirtualListModel_ViewUpdatesWithScroll(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}

	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	// Initial view
	viewBefore := model.View()

	// Scroll to middle
	model.SetSelected(50)

	// View should change
	viewAfter := model.View()

	assert.NotEqual(t, viewBefore, viewAfter, "View should change after scrolling")
}

// TestVirtualListModel_ViewWithBuffer tests buffer rendering around visible rows.
func TestVirtualListModel_ViewWithBuffer(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}

	renderFunc := func(item string, selected bool) string {
		return item
	}

	// Small viewport
	model := list.NewVirtualListModel(items, 10, 80, renderFunc)

	// Select item in middle
	model.SetSelected(50)

	view := model.View()

	// Should render more than just the 10 visible items (includes buffer)
	lines := strings.Split(view, "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// With buffer, should render a bit more than viewport height
	assert.Greater(t, nonEmptyLines, 10, "Should render buffer rows beyond viewport")
	assert.LessOrEqual(t, nonEmptyLines, 20, "Buffer should be reasonable size")
}

// TestVirtualListModel_ViewSelectedMarker tests selected item is visually marked.
func TestVirtualListModel_ViewSelectedMarker(t *testing.T) {
	items := []string{"apple", "banana", "cherry"}

	renderFunc := func(item string, selected bool) string {
		if selected {
			return "> " + item
		}
		return "  " + item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	// Select second item
	model.SetSelected(1)

	view := model.View()

	// View should contain the selected marker for "banana"
	assert.Contains(t, view, "> banana", "Selected item should have marker")
	assert.Contains(t, view, "  apple", "Unselected item should not have marker")
	assert.Contains(t, view, "  cherry", "Unselected item should not have marker")
}

// TestVirtualListModel_ViewEmptyList tests rendering with no items.
func TestVirtualListModel_ViewEmptyList(t *testing.T) {
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel([]string{}, 20, 80, renderFunc)

	view := model.View()

	// Empty view should return empty string or minimal content
	assert.Equal(t, "", view, "Empty list should produce empty view")
}

// TestVirtualListModel_ViewPerformance tests rendering performance with large lists.
func TestVirtualListModel_ViewPerformance(t *testing.T) {
	// Create 10,000 items (extreme case)
	items := make([]string, 10000)
	for i := range items {
		items[i] = "item"
	}

	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	// Render view (should be fast since it only renders visible rows)
	view := model.View()

	// Count rendered lines
	lines := strings.Split(view, "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// Even with 10,000 items, should only render ~20-30 rows
	assert.LessOrEqual(t, nonEmptyLines, 35, "Should render constant rows regardless of total items")
}

// TestVirtualListModel_ViewBoundaryConditions tests edge cases in rendering.
func TestVirtualListModel_ViewBoundaryConditions(t *testing.T) {
	tests := []struct {
		name       string
		totalItems int
		height     int
		selected   int
	}{
		{
			name:       "exactly viewport height items",
			totalItems: 20,
			height:     20,
			selected:   10,
		},
		{
			name:       "one item",
			totalItems: 1,
			height:     20,
			selected:   0,
		},
		{
			name:       "selected at start",
			totalItems: 100,
			height:     20,
			selected:   0,
		},
		{
			name:       "selected at end",
			totalItems: 100,
			height:     20,
			selected:   99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := make([]string, tt.totalItems)
			for i := range items {
				items[i] = "item"
			}

			renderFunc := func(item string, selected bool) string {
				return item
			}

			model := list.NewVirtualListModel(items, tt.height, 80, renderFunc)
			model.SetSelected(tt.selected)

			// Should not panic
			view := model.View()

			// Should produce some output
			assert.NotEmpty(t, view, "Should produce non-empty view")
		})
	}
}
