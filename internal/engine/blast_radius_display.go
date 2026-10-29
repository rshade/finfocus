package engine

import (
	"fmt"
	"strings"
)

// Blast-radius copy shared by the terminal and the web.
const (
	blastNoPrice       = "no price"
	blastNoCost        = "-"
	blastMixedCurrency = "total withheld: dependents use different currencies"
)

// applyBlastDisplay fills the display strings of an available result, so every
// surface renders the same text without formatting numbers itself.
func applyBlastDisplay(result *BlastRadius) {
	for i := range result.Dependents {
		d := &result.Dependents[i]
		d.MonthlyDisplay = blastMonthlyDisplay(*d)
		d.StatusLabel = blastStatusLabel(*d)
	}
	result.SummaryDisplay = blastSummaryDisplay(*result)
	result.NoteDisplay = blastNoteDisplay(*result)
}

func blastMonthlyDisplay(d BlastDependent) string {
	switch {
	case d.Component, d.Status == StatusDeleting:
		return blastNoCost
	case !d.Priced || d.MonthlyCost == nil:
		return blastNoPrice
	default:
		return FormatOverviewCurrency(*d.MonthlyCost)
	}
}

func blastStatusLabel(d BlastDependent) string {
	if d.Component {
		return "component"
	}
	switch d.Status {
	case StatusCreating:
		return "new"
	case StatusDeleting, StatusReplacing, StatusUpdating:
		return d.Status.String()
	case StatusActive:
		return ""
	}
	return ""
}

func blastSummaryDisplay(result BlastRadius) string {
	noun := "dependents"
	if result.Count == 1 {
		noun = "dependent"
	}
	if result.MonthlyTotal == nil {
		return fmt.Sprintf("%d %s", result.Count, noun)
	}
	return fmt.Sprintf("%d %s, %s/month", result.Count, noun, FormatOverviewCurrency(*result.MonthlyTotal))
}

// blastNoteDisplay says what the total leaves out, so a reader never takes it
// for the cost of every listed dependent.
func blastNoteDisplay(result BlastRadius) string {
	var notes []string
	var parts []string
	if result.LeftOut.Deleting > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", result.LeftOut.Deleting))
	}
	if result.LeftOut.Unpriced > 0 {
		parts = append(parts, fmt.Sprintf("%d unpriced", result.LeftOut.Unpriced))
	}
	if len(parts) > 0 {
		noun := "dependents are"
		if result.LeftOut.Deleting+result.LeftOut.Unpriced == 1 {
			noun = "dependent is"
		}
		notes = append(notes, strings.Join(parts, " and ")+" "+noun+" not in the total")
	}
	if result.LeftOut.Unresolved > 0 {
		notes = append(notes, fmt.Sprintf("%d unresolved references", result.LeftOut.Unresolved))
	}
	if result.MixedCurrency {
		notes = append(notes, blastMixedCurrency)
	}
	return strings.Join(notes, "; ")
}
