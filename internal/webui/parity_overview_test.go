package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

func TestOverviewQueryParityWithCLIJSON(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../testdata/simple-plan.json")
	require.NoError(t, err)
	plan, err := ingest.ParsePulumiPlan(data)
	require.NoError(t, err)
	steps := make([]engine.PlanStep, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(
			steps,
			engine.PlanStep{URN: step.URN, Type: step.Type, Op: step.Op, ProjectedProperties: step.Inputs},
		)
	}
	rows, err := engine.MergeResourcesForOverview(context.Background(), nil, steps)
	require.NoError(t, err)
	result := engine.ComputeOverviewResult(rows, 7)
	var cli bytes.Buffer
	require.NoError(
		t,
		engine.RenderOverviewAsJSON(context.Background(), &cli, result, engine.StackContext{StackName: "fixture"}, nil),
	)
	var canonical engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(cli.Bytes(), &canonical))
	session := NewSession(context.Background(), SessionOptions{DayOfMonth: 7})
	session.SetData(nil, rows, engine.DateRange{}, "fixture")
	s := newRunningServer(t, Options{Session: session})
	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{"sort":"name"}`))
	require.Len(t, page.Rows, len(canonical.Resources))
	sources := make(map[string]engine.OverviewRow, len(page.Rows))
	for _, row := range page.Rows {
		sources[row.URN] = row.Source
	}
	for _, row := range canonical.Resources {
		assert.Equal(t, row, sources[row.URN])
	}
	assert.InDelta(t, canonical.Summary.TotalActualMTD, page.Totals.TotalActual, 1e-9)
	assert.InDelta(t, canonical.Summary.ProjectedMonthly, page.Totals.TotalProjected, 1e-9)
	assert.InDelta(t, canonical.Summary.ProjectedDelta, page.Totals.TotalDelta, 1e-9)
	assert.InDelta(t, canonical.Summary.PotentialSavings, page.Totals.TotalSavings, 1e-9)
	assert.Equal(t, canonical.Summary.Currency, page.Totals.Currency)
}
