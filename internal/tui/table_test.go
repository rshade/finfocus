package tui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/stretchr/testify/assert"
)

func TestDefaultTableStyles(t *testing.T) {
	s := DefaultTableStyles()
	// Check if our styles are applied
	// Note: lipgloss.Style equality checking is not straightforward,
	// but we can check if the underlying values we set are present.
	// For now, ensuring it returns a struct without panic is a good start,
	// and we can check if it differs from empty default.
	assert.NotEqual(t, table.Styles{}, s)
}

func TestNewTable(t *testing.T) {
	cols := []table.Column{{Title: "Test", Width: 10}}
	rows := []table.Row{{"Data"}}
	height := 5

	tbl := NewTable(cols, rows, height)
	assert.Equal(t, cols, tbl.Columns())
	assert.Equal(t, rows, tbl.Rows())
	// bubbles/table subtracts the header height (1) from the total height
	// to determine the viewport height.
	assert.Equal(t, height-1, tbl.Height())
	assert.True(t, tbl.Focused())
}
