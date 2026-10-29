package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlastRadiusCopy(t *testing.T) {
	t.Parallel()

	total := 10.0
	tests := []struct {
		name   string
		radius BlastRadius
		want   string
		got    func(BlastRadius) string
	}{
		{"summary one", BlastRadius{Count: 1, MonthlyTotal: &total}, "1 dependent, $10.00/month", blastSummaryDisplay},
		{
			"summary many",
			BlastRadius{Count: 2, MonthlyTotal: &total},
			"2 dependents, $10.00/month",
			blastSummaryDisplay,
		},
		{
			"summary usd",
			BlastRadius{Count: 2, MonthlyTotal: &total, Currency: "USD"},
			"2 dependents, $10.00/month",
			blastSummaryDisplay,
		},
		{
			"summary other currency",
			BlastRadius{Count: 2, MonthlyTotal: &total, Currency: "EUR"},
			"2 dependents, $10.00 EUR/month",
			blastSummaryDisplay,
		},
		{"summary withheld", BlastRadius{Count: 2}, "2 dependents", blastSummaryDisplay},
		{
			"left out one",
			BlastRadius{LeftOut: BlastLeftOut{Deleting: 1}},
			"1 deleted dependent is not in the total",
			blastNoteDisplay,
		},
		{
			"left out both",
			BlastRadius{LeftOut: BlastLeftOut{Deleting: 1, Unpriced: 2}},
			"1 deleted and 2 unpriced dependents are not in the total",
			blastNoteDisplay,
		},
		{
			"unresolved one",
			BlastRadius{LeftOut: BlastLeftOut{Unresolved: 1}},
			"1 unresolved reference",
			blastNoteDisplay,
		},
		{"unresolved", BlastRadius{LeftOut: BlastLeftOut{Unresolved: 3}}, "3 unresolved references", blastNoteDisplay},
		{"mixed currency", BlastRadius{MixedCurrency: true}, blastMixedCurrency, blastNoteDisplay},
		{"none left out", BlastRadius{Count: 2}, "", blastNoteDisplay},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.got(tt.radius))
		})
	}
}

func TestBlastRadiusDependentDisplay(t *testing.T) {
	t.Parallel()

	cost := 1234.5
	tests := []struct {
		name      string
		dependent BlastDependent
		monthly   string
		label     string
	}{
		{"priced active", BlastDependent{Status: StatusActive, Priced: true, MonthlyCost: &cost}, "$1,234.50", ""},
		{"priced new", BlastDependent{Status: StatusCreating, Priced: true, MonthlyCost: &cost}, "$1,234.50", "new"},
		{"unpriced", BlastDependent{Status: StatusUpdating}, "no price", "updating"},
		{"deleting", BlastDependent{Status: StatusDeleting}, "-", "deleting"},
		{
			"replacing",
			BlastDependent{Status: StatusReplacing, Priced: true, MonthlyCost: &cost},
			"$1,234.50",
			"replacing",
		},
		{"component", BlastDependent{Component: true}, "-", "component"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.monthly, blastMonthlyDisplay(tt.dependent))
			assert.Equal(t, tt.label, blastStatusLabel(tt.dependent))
		})
	}
}

// A dependent with no overview row takes its type from the graph, and one the
// preview creates is labelled new and counted as unpriced.
func TestComputeBlastRadius_DependentWithoutRow(t *testing.T) {
	t.Parallel()

	root := blastURN("aws:rds/instance:Instance", "db")
	created := blastURN("aws:ecs/service:Service", "api")
	group := blastURN("my:index:Group", "group")
	graph := blastGraph(
		DependencyNode{URN: root, Type: "aws:rds/instance:Instance"},
		DependencyNode{
			URN: created, Type: "aws:ecs/service:Service", DependsOn: []string{root}, Status: StatusCreating,
		},
		DependencyNode{URN: group, Type: "my:index:Group", DependsOn: []string{root}, Component: true},
	)

	got := ComputeBlastRadius(graph, nil, root)

	byURN := map[string]BlastDependent{}
	for _, d := range got.Dependents {
		byURN[d.URN] = d
	}
	assert.Equal(t, "aws:ecs/service:Service", byURN[created].Type)
	assert.Equal(t, StatusCreating, byURN[created].Status)
	assert.Equal(t, "new", byURN[created].StatusLabel)
	assert.Equal(t, "no price", byURN[created].MonthlyDisplay)
	assert.Equal(t, "my:index:Group", byURN[group].Type)
	assert.Equal(t, "component", byURN[group].StatusLabel)
	assert.Equal(t, 1, got.Count)
	assert.Equal(t, BlastLeftOut{Unpriced: 1}, got.LeftOut)
	assert.Equal(t, "1 dependent", got.SummaryDisplay)
	assert.Equal(t, "1 unpriced dependent is not in the total", got.NoteDisplay)
}
