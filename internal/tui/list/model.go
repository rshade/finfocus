package listview

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// defaultBufferSize is the number of extra rows to render above/below viewport for smooth scrolling.
const defaultBufferSize = 5

// halfViewportDivisor is used to calculate half the viewport height for centering.
const halfViewportDivisor = 2

// RenderFunc is a function that renders an item at a given index.
// The selected parameter indicates whether this item is currently selected.
type RenderFunc[T any] func(item T, selected bool) string

// VirtualListModel implements virtual scrolling for large lists.
// It renders only the visible portion of the list plus a small buffer,
// allowing smooth scrolling with 10,000+ items without performance degradation.
type VirtualListModel[T any] struct {
	// items contains all list items
	items []T

	// renderFunc renders a single item
	renderFunc RenderFunc[T]

	// selected is the currently selected item index (0-based)
	selected int

	// visibleFrom is the first visible item index
	visibleFrom int

	// visibleTo is the last visible item index (exclusive)
	visibleTo int

	// height is the viewport height in rows
	height int

	// width is the viewport width in columns
	width int

	// bufferSize is the number of extra rows to render above/below viewport
	bufferSize int
}

// NewVirtualListModel creates a new virtual list model.
// items: the complete list of items to display.
// height: viewport height in rows.
// width: viewport width in columns.
// renderFunc: function to render each item.
func NewVirtualListModel[T any](items []T, height, width int, renderFunc RenderFunc[T]) *VirtualListModel[T] {
	m := &VirtualListModel[T]{
		items:      items,
		renderFunc: renderFunc,
		selected:   0,
		height:     height,
		width:      width,
		bufferSize: defaultBufferSize, // Render extra rows above/below for smooth scrolling
	}

	m.updateVisibleRange()
	return m
}

// Init initializes the model (required for tea.Model interface).
func (m *VirtualListModel[T]) Init() tea.Cmd {
	return nil
}

// Update handles keyboard and resize messages.
func (m *VirtualListModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg), nil
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.updateVisibleRange()
		return m, nil
	}

	return m, nil
}

// handleKeyMsg processes keyboard input for navigation.
//
//nolint:gocognit,exhaustive // Key handling inherently requires multiple branches for different navigation keys.
func (m *VirtualListModel[T]) handleKeyMsg(msg tea.KeyMsg) tea.Model {
	if len(m.items) == 0 {
		return m
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.selected > 0 {
			m.selected--
			m.updateVisibleRange()
		}

	case tea.KeyDown:
		if m.selected < len(m.items)-1 {
			m.selected++
			m.updateVisibleRange()
		}

	case tea.KeyPgUp:
		m.selected -= m.height
		if m.selected < 0 {
			m.selected = 0
		}
		m.updateVisibleRange()

	case tea.KeyPgDown:
		m.selected += m.height
		if m.selected >= len(m.items) {
			m.selected = len(m.items) - 1
		}
		m.updateVisibleRange()

	case tea.KeyHome:
		m.selected = 0
		m.updateVisibleRange()

	case tea.KeyEnd:
		m.selected = len(m.items) - 1
		m.updateVisibleRange()

	case tea.KeyRunes:
		// Handle vim-style navigation
		if len(msg.Runes) > 0 {
			switch msg.Runes[0] {
			case 'j':
				if m.selected < len(m.items)-1 {
					m.selected++
					m.updateVisibleRange()
				}
			case 'k':
				if m.selected > 0 {
					m.selected--
					m.updateVisibleRange()
				}
			}
		}

	default:
		// Ignore other key types (Ctrl combinations, function keys, etc.)
	}

	return m
}

// updateVisibleRange calculates the visible range of items based on selection and viewport.
// This ensures the selected item is always visible and updates visibleFrom/visibleTo.
func (m *VirtualListModel[T]) updateVisibleRange() {
	if len(m.items) == 0 {
		m.visibleFrom = 0
		m.visibleTo = 0
		return
	}

	// Calculate the ideal visible range centered on the selected item
	halfViewport := m.height / halfViewportDivisor

	// Start by centering the selected item
	idealFrom := m.selected - halfViewport
	idealTo := m.selected + halfViewport

	// Adjust if we're near the start
	if idealFrom < 0 {
		idealFrom = 0
		idealTo = m.height
	}

	// Adjust if we're near the end
	if idealTo > len(m.items) {
		idealTo = len(m.items)
		idealFrom = idealTo - m.height
		if idealFrom < 0 {
			idealFrom = 0
		}
	}

	// Store the viewport range (without buffer)
	m.visibleFrom = idealFrom
	m.visibleTo = idealTo
}

// View renders the visible portion of the list with buffer.
func (m *VirtualListModel[T]) View() string {
	if len(m.items) == 0 {
		return ""
	}

	// Calculate render range with buffer for smooth scrolling
	renderFrom := m.visibleFrom - m.bufferSize
	if renderFrom < 0 {
		renderFrom = 0
	}

	renderTo := m.visibleTo + m.bufferSize
	if renderTo > len(m.items) {
		renderTo = len(m.items)
	}

	var content string
	var contentSb199 strings.Builder
	for i := renderFrom; i < renderTo; i++ {
		isSelected := i == m.selected
		line := m.renderFunc(m.items[i], isSelected)
		contentSb199.WriteString(line + "\n")
	}
	content += contentSb199.String()

	// Remove trailing newline
	if len(content) > 0 {
		content = content[:len(content)-1]
	}

	return content
}

// ItemCount returns the total number of items in the list.
func (m *VirtualListModel[T]) ItemCount() int {
	return len(m.items)
}

// Selected returns the currently selected item index.
func (m *VirtualListModel[T]) Selected() int {
	return m.selected
}

// SetSelected sets the selected item index, capping to valid bounds.
func (m *VirtualListModel[T]) SetSelected(index int) {
	if len(m.items) == 0 {
		m.selected = 0
		return
	}

	switch {
	case index < 0:
		m.selected = 0
	case index >= len(m.items):
		m.selected = len(m.items) - 1
	default:
		m.selected = index
	}

	m.updateVisibleRange()
}

// VisibleFrom returns the first visible item index (inclusive).
func (m *VirtualListModel[T]) VisibleFrom() int {
	return m.visibleFrom
}

// VisibleTo returns the last visible item index (exclusive).
func (m *VirtualListModel[T]) VisibleTo() int {
	return m.visibleTo
}

// Height returns the viewport height.
func (m *VirtualListModel[T]) Height() int {
	return m.height
}

// Width returns the viewport width.
func (m *VirtualListModel[T]) Width() int {
	return m.width
}

// GetSelectedItem returns the currently selected item.
// Returns nil if list is empty.
func (m *VirtualListModel[T]) GetSelectedItem() *T {
	if len(m.items) == 0 || m.selected < 0 || m.selected >= len(m.items) {
		return nil
	}
	return &m.items[m.selected]
}
