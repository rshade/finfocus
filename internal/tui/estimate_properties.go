package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func (m *EstimateModel) buildPlan(
	overrides map[string]string,
	hints []engine.PricingMetricHint,
) viewmodel.EstimatePropertyPlan {
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return viewmodel.BuildEstimatePropertyPlan(ctx, m.resource, overrides, hints)
}

// Properties returns every editable row in editor order, whether or not it is
// currently shown. The caller owns the copy.
func (m *EstimateModel) Properties() []PropertyRow {
	return append([]PropertyRow{}, m.properties...)
}

// Details returns the read-only identity rows. The caller owns the copy.
func (m *EstimateModel) Details() []viewmodel.EstimateDetail {
	return append([]viewmodel.EstimateDetail{}, m.details...)
}

func (m *EstimateModel) applyPlan(plan viewmodel.EstimatePropertyPlan) {
	m.properties = plan.Rows
	m.details = plan.Details
	m.note = plan.Note
}

// visibleIndexes lists the rows the cursor can reach: pricing inputs, edited
// rows, every row of an uncurated type, and all rows while show-all is on.
// Overrides, recalculation, and results always read every row.
func (m *EstimateModel) visibleIndexes() []int {
	indexes := make([]int, 0, len(m.properties))
	for i, row := range m.properties {
		if m.showAll || m.note != "" || row.Class == viewmodel.PropertyClassPricing ||
			row.CurrentValue != row.OriginalValue {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m *EstimateModel) focusedPropertyIndex() (int, bool) {
	visible := m.visibleIndexes()
	if m.focusedRow < 0 || m.focusedRow >= len(visible) {
		return 0, false
	}
	return visible[m.focusedRow], true
}

func (m *EstimateModel) focusedKey() (string, bool) {
	index, ok := m.focusedPropertyIndex()
	if !ok {
		return "", false
	}
	return m.properties[index].Key, true
}

func (m *EstimateModel) restoreFocus(key string, had bool) {
	visible := m.visibleIndexes()
	if had {
		for position, index := range visible {
			if m.properties[index].Key == key {
				m.focusedRow = position
				return
			}
		}
	}
	m.focusedRow = max(0, min(m.focusedRow, len(visible)-1))
	if !had {
		m.focusedRow = 0
	}
}

func (m *EstimateModel) toggleShowAll() {
	key, had := m.focusedKey()
	m.showAll = !m.showAll
	m.restoreFocus(key, had)
}

// replan rebuilds the rows when discovery delivers usage hints after the model
// was built. Typed and committed edits, the deltas, and the cursor row stay by key.
func (m *EstimateModel) replan(hints []engine.PricingMetricHint) {
	key, had := m.focusedKey()
	plan := m.buildPlan(m.GetOverrides(), hints)
	m.applyPlan(plan)
	viewmodel.ApplyEstimateDeltas(m.properties, m.deltas)
	m.restoreFocus(key, had)
}

func (m *EstimateModel) visibleRows() []PropertyRow {
	visible := m.visibleIndexes()
	rows := make([]PropertyRow, 0, len(visible))
	for _, index := range visible {
		rows = append(rows, m.properties[index])
	}
	return rows
}

func (m *EstimateModel) renderPropertySection() string {
	rows := m.visibleRows()
	if m.editMode && m.focusedRow < len(rows) {
		rows[m.focusedRow].CurrentValue = m.editBuffer + "▌"
		return RenderPropertyTable(rows, m.focusedRow, true)
	}
	return RenderPropertyTable(rows, m.focusedRow, false)
}

// RenderReadOnlyDetails renders identity metadata that has no cursor and no edit.
func RenderReadOnlyDetails(details []viewmodel.EstimateDetail) string {
	if len(details) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(ColorHeader).Bold(true).Render("Read-only:"))
	for _, detail := range details {
		fmt.Fprintf(&sb, "\n  %s: %s", detail.Key, detail.Value)
	}
	return sb.String()
}

func renderEstimateNote(note string) string {
	if note == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).Render(note)
}
