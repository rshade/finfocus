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
	// bubbles/v2 viewport requires non-zero width to render rows.
	assert.Equal(t, 10+defaultCellPadding, tbl.Width())
}

func TestTableWidthFromColumns(t *testing.T) {
	tests := []struct {
		name    string
		columns []table.Column
		want    int
	}{
		{
			name:    "single column",
			columns: []table.Column{{Title: "A", Width: 10}},
			want:    12, // 10 + 2 padding
		},
		{
			name: "multiple columns",
			columns: []table.Column{
				{Title: "A", Width: 10},
				{Title: "B", Width: 20},
				{Title: "C", Width: 5},
			},
			want: 41, // (10+2) + (20+2) + (5+2)
		},
		{
			name:    "empty columns",
			columns: []table.Column{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tableWidthFromColumns(tt.columns))
		})
	}
}

func TestTablePaddingForColumns(t *testing.T) {
	tests := []struct {
		name        string
		columnCount int
		want        int
	}{
		{
			name:        "no columns",
			columnCount: 0,
			want:        0,
		},
		{
			name:        "single column",
			columnCount: 1,
			want:        defaultCellPadding,
		},
		{
			name:        "multiple columns",
			columnCount: 8,
			want:        8 * defaultCellPadding,
		},
		{
			name:        "negative count",
			columnCount: -1,
			want:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tablePaddingForColumns(tt.columnCount))
		})
	}
}
