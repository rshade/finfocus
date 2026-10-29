package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	blastUnavailableMsg = "  dependency information is not available"
	blastEmptyMsg       = "  nothing depends on this resource"
	blastIndentStep     = "  "
	blastBranchGlyph    = "└ "
	blastCostGap        = 2
)

// blastRadiusFor computes the blast radius of row from the rows the model
// holds and the graph it was last sent.
func (m OverviewModel) blastRadiusFor(row engine.OverviewRowResult) engine.BlastRadius {
	rows := make([]engine.OverviewRow, len(m.allRows))
	for i := range m.allRows {
		rows[i] = m.allRows[i].Source
	}
	return engine.ComputeBlastRadius(m.graph, rows, row.URN)
}

// renderDetailBlastRadius writes the BLAST RADIUS section. The count and
// total come first so scrolling the detail view never hides them.
func renderDetailBlastRadius(content *strings.Builder, radius engine.BlastRadius) {
	content.WriteString(HeaderStyle.Render("BLAST RADIUS"))
	content.WriteString("\n")
	switch {
	case !radius.Available:
		content.WriteString(SubtleStyle.Render(blastUnavailableMsg))
		content.WriteString("\n\n")
		return
	case len(radius.Dependents) == 0:
		content.WriteString(SubtleStyle.Render(blastEmptyMsg))
		content.WriteString("\n\n")
		return
	}

	content.WriteString(ValueStyle.Render("  " + radius.SummaryDisplay))
	content.WriteString("\n")

	dependents := blastTreeOrder(radius)
	labels := make([]string, len(dependents))
	width := 0
	for i, d := range dependents {
		labels[i] = blastLabel(d)
		width = max(width, utf8.RuneCountInString(labels[i]))
	}
	for i, d := range dependents {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(labels[i])+blastCostGap)
		fmt.Fprintf(content, "  %s%s%s\n", labels[i], pad, blastCost(d))
	}
	if radius.NoteDisplay != "" {
		content.WriteString(SubtleStyle.Render("  " + radius.NoteDisplay))
		content.WriteString("\n")
	}
	content.WriteString("\n")
}

// blastTreeOrder reorders the dependents, which the engine lists by distance,
// so each one follows its tree parent (Via) and the indent reads as nesting.
func blastTreeOrder(radius engine.BlastRadius) []engine.BlastDependent {
	children := make(map[string][]engine.BlastDependent, len(radius.Dependents))
	for _, d := range radius.Dependents {
		children[d.Via] = append(children[d.Via], d)
	}
	ordered := make([]engine.BlastDependent, 0, len(radius.Dependents))
	var walk func(parent string)
	walk = func(parent string) {
		for _, d := range children[parent] {
			ordered = append(ordered, d)
			walk(d.URN)
		}
	}
	walk(radius.URN)
	return ordered
}

// blastCost is the cost cell followed by the status label, when there is one.
func blastCost(d engine.BlastDependent) string {
	if d.StatusLabel != "" {
		return d.MonthlyDisplay + "  [" + d.StatusLabel + "]"
	}
	return d.MonthlyDisplay
}

func blastLabel(d engine.BlastDependent) string {
	indent := strings.Repeat(blastIndentStep, d.Depth-1)
	return fmt.Sprintf("%s%s%s (%s)", indent, blastBranchGlyph, d.DisplayName, d.Type)
}
