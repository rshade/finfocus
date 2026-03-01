package tui

import (
	"charm.land/bubbles/v2/table"
)

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
	)
	t.SetStyles(DefaultTableStyles())
	return t
}
