package tui

import (
	"charm.land/bubbles/v2/table"
)

// defaultCellPadding is the total horizontal padding added by the default Cell
// style (Padding(0, 1) = 1 left + 1 right).
const defaultCellPadding = 2

// tableWidthFromColumns computes the total rendered width of a table row from
// column definitions, accounting for default cell padding.
func tableWidthFromColumns(columns []table.Column) int {
	w := 0
	for _, col := range columns {
		w += col.Width + defaultCellPadding
	}
	return w
}

// tablePaddingForColumns returns the total horizontal padding introduced by
// the default cell style for a given column count.
func tablePaddingForColumns(columnCount int) int {
	if columnCount <= 0 {
		return 0
	}
	return columnCount * defaultCellPadding
}

// DefaultTableStyles returns a table.Styles with standardized header and selection styles applied.
// It extends table.DefaultStyles() by setting Header to TableHeaderStyle and Selected to TableSelectedStyle.
func DefaultTableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = TableHeaderStyle
	s.Selected = TableSelectedStyle
	return s
}

// NewTable creates a focused table.Model with the given columns, rows, and visible height,
// applying DefaultTableStyles for consistent styling.
func NewTable(columns []table.Column, rows []table.Row, height int) table.Model {
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithWidth(tableWidthFromColumns(columns)),
	)
	t.SetStyles(DefaultTableStyles())
	return t
}
