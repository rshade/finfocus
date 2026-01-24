package list_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/tui/list"
)

// TestVirtualListModel_NewModel tests VirtualListModel initialization.
func TestVirtualListModel_NewModel(t *testing.T) {
	items := []string{"item1", "item2", "item3", "item4", "item5"}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	assert.Equal(t, 5, model.ItemCount())
	assert.Equal(t, 20, model.Height())
	assert.Equal(t, 80, model.Width())
	assert.Equal(t, 0, model.Selected())
	assert.Equal(t, 0, model.VisibleFrom())
}

// TestVirtualListModel_VisibleRangeCalculation tests visible range logic.
func TestVirtualListModel_VisibleRangeCalculation(t *testing.T) {
	tests := []struct {
		name           string
		totalItems     int
		viewportHeight int
		selectedIndex  int
		expectFrom     int
		expectTo       int
	}{
		{
			name:           "first page with 100 items",
			totalItems:     100,
			viewportHeight: 20,
			selectedIndex:  0,
			expectFrom:     0,
			expectTo:       20,
		},
		{
			name:           "middle page with 100 items",
			totalItems:     100,
			viewportHeight: 20,
			selectedIndex:  50,
			expectFrom:     40,
			expectTo:       60,
		},
		{
			name:           "last page with 100 items",
			totalItems:     100,
			viewportHeight: 20,
			selectedIndex:  99,
			expectFrom:     80,
			expectTo:       100,
		},
		{
			name:           "fewer items than viewport",
			totalItems:     10,
			viewportHeight: 20,
			selectedIndex:  5,
			expectFrom:     0,
			expectTo:       10,
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

			model := list.NewVirtualListModel(items, tt.viewportHeight, 80, renderFunc)
			model.SetSelected(tt.selectedIndex)

			assert.Equal(t, tt.expectFrom, model.VisibleFrom())
			assert.Equal(t, tt.expectTo, model.VisibleTo())
		})
	}
}

// TestVirtualListModel_ScrollBoundaries tests scroll boundary conditions.
func TestVirtualListModel_ScrollBoundaries(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	tests := []struct {
		name           string
		action         func() tea.Msg
		expectSelected int
		expectInBounds bool
	}{
		{
			name: "down from start",
			action: func() tea.Msg {
				return tea.KeyMsg{Type: tea.KeyDown}
			},
			expectSelected: 1,
			expectInBounds: true,
		},
		{
			name: "up at start stays at 0",
			action: func() tea.Msg {
				model.SetSelected(0)
				return tea.KeyMsg{Type: tea.KeyUp}
			},
			expectSelected: 0,
			expectInBounds: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.action()
			_, _ = model.Update(msg)

			assert.Equal(t, tt.expectSelected, model.Selected())

			// Verify selected is within bounds
			if tt.expectInBounds {
				assert.GreaterOrEqual(t, model.Selected(), 0)
				assert.Less(t, model.Selected(), model.ItemCount())
			}
		})
	}
}

// TestVirtualListModel_SelectionLogic tests item selection behavior.
func TestVirtualListModel_SelectionLogic(t *testing.T) {
	items := make([]string, 50)
	for i := range items {
		items[i] = "item"
	}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	tests := []struct {
		name          string
		key           tea.KeyMsg
		initialIndex  int
		expectedIndex int
	}{
		{
			name:          "down arrow moves forward",
			key:           tea.KeyMsg{Type: tea.KeyDown},
			initialIndex:  5,
			expectedIndex: 6,
		},
		{
			name:          "up arrow moves backward",
			key:           tea.KeyMsg{Type: tea.KeyUp},
			initialIndex:  10,
			expectedIndex: 9,
		},
		{
			name:          "j key moves forward",
			key:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
			initialIndex:  5,
			expectedIndex: 6,
		},
		{
			name:          "k key moves backward",
			key:           tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
			initialIndex:  10,
			expectedIndex: 9,
		},
		{
			name:          "home key goes to start",
			key:           tea.KeyMsg{Type: tea.KeyHome},
			initialIndex:  25,
			expectedIndex: 0,
		},
		{
			name:          "end key goes to last",
			key:           tea.KeyMsg{Type: tea.KeyEnd},
			initialIndex:  5,
			expectedIndex: 49,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.SetSelected(tt.initialIndex)
			_, _ = model.Update(tt.key)

			assert.Equal(t, tt.expectedIndex, model.Selected())
		})
	}
}

// TestVirtualListModel_PageUpDown tests page navigation.
func TestVirtualListModel_PageUpDown(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	tests := []struct {
		name          string
		key           tea.KeyMsg
		initialIndex  int
		expectedIndex int
	}{
		{
			name:          "page down moves viewport height",
			key:           tea.KeyMsg{Type: tea.KeyPgDown},
			initialIndex:  10,
			expectedIndex: 30, // +20 (viewport height)
		},
		{
			name:          "page up moves viewport height",
			key:           tea.KeyMsg{Type: tea.KeyPgUp},
			initialIndex:  50,
			expectedIndex: 30, // -20 (viewport height)
		},
		{
			name:          "page down at end caps to last",
			key:           tea.KeyMsg{Type: tea.KeyPgDown},
			initialIndex:  90,
			expectedIndex: 99, // Capped to last item
		},
		{
			name:          "page up at start caps to first",
			key:           tea.KeyMsg{Type: tea.KeyPgUp},
			initialIndex:  5,
			expectedIndex: 0, // Capped to first item
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.SetSelected(tt.initialIndex)
			_, _ = model.Update(tt.key)

			assert.Equal(t, tt.expectedIndex, model.Selected())
		})
	}
}

// TestVirtualListModel_WindowResize tests viewport height adjustment.
func TestVirtualListModel_WindowResize(t *testing.T) {
	items := make([]string, 100)
	for i := range items {
		items[i] = "item"
	}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	// Simulate window resize
	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 30}
	_, _ = model.Update(resizeMsg)

	assert.Equal(t, 30, model.Height())
	assert.Equal(t, 120, model.Width())
}

// TestVirtualListModel_EmptyList tests behavior with no items.
func TestVirtualListModel_EmptyList(t *testing.T) {
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel([]string{}, 20, 80, renderFunc)

	assert.Equal(t, 0, model.ItemCount())
	assert.Equal(t, 0, model.Selected())
	assert.Equal(t, 0, model.VisibleFrom())
	assert.Equal(t, 0, model.VisibleTo())

	// Verify navigation doesn't panic with empty list
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
}

// TestVirtualListModel_SingleItem tests behavior with one item.
func TestVirtualListModel_SingleItem(t *testing.T) {
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel([]string{"item1"}, 20, 80, renderFunc)

	assert.Equal(t, 1, model.ItemCount())
	assert.Equal(t, 0, model.Selected())

	// Navigation should keep selection at 0
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 0, model.Selected())

	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, model.Selected())
}

// TestVirtualListModel_SetSelected tests direct selection setting.
func TestVirtualListModel_SetSelected(t *testing.T) {
	items := make([]string, 50)
	for i := range items {
		items[i] = "item"
	}
	renderFunc := func(item string, selected bool) string {
		return item
	}

	model := list.NewVirtualListModel(items, 20, 80, renderFunc)

	tests := []struct {
		name          string
		setIndex      int
		expectedIndex int
		description   string
	}{
		{
			name:          "valid index",
			setIndex:      25,
			expectedIndex: 25,
			description:   "Should set to valid index",
		},
		{
			name:          "negative index caps to 0",
			setIndex:      -5,
			expectedIndex: 0,
			description:   "Negative should cap to 0",
		},
		{
			name:          "beyond bounds caps to last",
			setIndex:      100,
			expectedIndex: 49,
			description:   "Beyond bounds should cap to last",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.SetSelected(tt.setIndex)
			assert.Equal(t, tt.expectedIndex, model.Selected(), tt.description)
		})
	}
}
