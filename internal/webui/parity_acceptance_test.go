package webui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/tui"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// The three scrubbed stack exports exercise different counts, types and cluster
// lineage. Nonzero enrichment deliberately catches lost rows and totals. The
// real-process browser suite separately exercises the live plugin transport.
func TestThreeFixtureTransportParity(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"state-no-changes.json", "state-mixed-changes.json", "state-cluster-expansion.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rows, resources, costs, recs := parityFixture(t, name)
			ctx := context.Background()
			eng := engine.New([]*pluginhost.Client{{Name: "fixture", API: &webEstimatePlugin{rate: 10}}}, nil)
			session := NewSession(ctx, SessionOptions{
				DayOfMonth: 7,
				ActualCosts: func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
					return &engine.CostResultWithErrors{Results: costs}, nil
				},
				Recommendations: func(context.Context, bool) (*engine.RecommendationsResult, error) {
					return &engine.RecommendationsResult{Recommendations: recs}, nil
				},
				EstimateResources: func(context.Context) ([]engine.ResourceDescriptor, error) { return resources, nil },
			})
			session.SetData(eng, rows, engine.DateRange{}, name)
			server := newRunningServer(t, Options{Session: session})
			parityOverview(t, server, rows)
			parityActual(t, server, costs)
			parityRecommendations(t, server, recs)
			parityEstimate(t, server, eng, resources[0])
		})
	}
}

func parityFixture(
	t *testing.T,
	name string,
) ([]engine.OverviewRow, []engine.ResourceDescriptor, []engine.CostResult, []engine.Recommendation) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/overview/" + name)
	require.NoError(t, err)
	state, err := ingest.ParseStackExport(data)
	require.NoError(t, err)
	descriptors, err := ingest.MapStateResources(state.GetCustomResources())
	require.NoError(t, err)
	require.NotEmpty(t, descriptors)
	rows := make([]engine.OverviewRow, 0, len(descriptors))
	costs := make([]engine.CostResult, 0, len(descriptors))
	recs := make([]engine.Recommendation, 0, len(descriptors))
	risk := 0.2
	for i := range descriptors {
		r := &descriptors[i]
		r.Properties = map[string]any{"size": float64(i + 1)}
		amount := float64(i+1) * 12.5
		rec := engine.Recommendation{
			ID:               fmt.Sprintf("rec-%d", i),
			ResourceID:       r.ID,
			Type:             "RIGHTSIZE",
			Description:      "Resize fixture",
			EstimatedSavings: amount / 4,
			Currency:         "USD",
			Scores:           &engine.RecommendationScores{Risk: &risk},
		}
		recs = append(recs, rec)
		cost := engine.CostResult{
			ResourceID:      r.ID,
			ResourceType:    r.Type,
			TotalCost:       amount,
			Monthly:         amount * 2,
			Currency:        "USD",
			Breakdown:       map[string]float64{"compute": amount * .8, "storage": amount * .2},
			Recommendations: []engine.Recommendation{rec},
		}
		costs = append(costs, cost)
		rows = append(
			rows,
			engine.OverviewRow{
				URN:             r.ID,
				Type:            r.Type,
				Status:          engine.StatusActive,
				Properties:      r.Properties,
				ActualCost:      &engine.ActualCostData{MTDCost: amount, Currency: "USD", Breakdown: cost.Breakdown},
				ProjectedCost:   &engine.ProjectedCostData{MonthlyCost: amount * 2, Currency: "USD"},
				Recommendations: []engine.Recommendation{rec},
			},
		)
	}
	return rows, descriptors, costs, recs
}

func parityOverview(t *testing.T, s *Server, rows []engine.OverviewRow) {
	t.Helper()
	result := engine.ComputeOverviewResult(rows, 7)
	var cli bytes.Buffer
	require.NoError(t, engine.RenderOverviewAsJSON(context.Background(), &cli, result, engine.StackContext{}, nil))
	var canonical engine.OverviewJSONOutput
	require.NoError(t, json.Unmarshal(cli.Bytes(), &canonical))
	page := decodeOverviewPage(t, overviewRequest(t, s, http.MethodPost, "/api/overview/query", `{"sort":"name"}`))
	require.Len(t, page.Rows, len(canonical.Resources))
	byURN := map[string]engine.OverviewRow{}
	for _, row := range page.Rows {
		byURN[row.URN] = row.Source
	}
	for _, row := range canonical.Resources {
		assert.Equal(t, row, byURN[row.URN])
	}
	assert.InDelta(t, canonical.Summary.TotalActualMTD, page.Totals.TotalActual, 1e-9)
	assert.InDelta(t, canonical.Summary.ProjectedMonthly, page.Totals.TotalProjected, 1e-9)
	assert.InDelta(t, canonical.Summary.PotentialSavings, page.Totals.TotalSavings, 1e-9)
	model, _ := tui.NewOverviewModel(context.Background(), rows, len(rows), nil, nil)
	updated, _ := model.Update(tui.OverviewAllResourcesLoadedMsg{})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 300, Height: 200})
	text := ansi.Strip(updated.View().Content)
	for _, row := range page.Rows {
		assert.True(
			t,
			parityOverviewRowMatches(text, row),
			"resource %s must render its own actual/projected/delta cells",
			row.URN,
		)
	}
}

func parityActual(t *testing.T, s *Server, costs []engine.CostResult) {
	t.Helper()
	var cli bytes.Buffer
	require.NoError(t, engine.RenderActualCostResults(&cli, engine.OutputJSON, costs, false))
	var canonical []engine.CostResult
	require.NoError(t, json.Unmarshal(cli.Bytes(), &canonical))
	response := overviewRequest(t, s, http.MethodPost, "/api/cost/actual/query", `{}`)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var page struct {
		Results []engine.CostResult         `json:"results"`
		Summary viewmodel.ActualCostSummary `json:"summary"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&page))
	assert.ElementsMatch(t, canonical, page.Results)
	wantSummary := engine.AggregateResults(costs).Summary
	assert.ElementsMatch(t, wantSummary.Resources, page.Summary.Resources)
	wantSummary.Resources = nil
	gotSummary := page.Summary.CostSummary
	assert.InDelta(t, wantSummary.TotalMonthly, gotSummary.TotalMonthly, 1e-9)
	assert.InDelta(t, wantSummary.TotalHourly, gotSummary.TotalHourly, 1e-9)
	assert.Equal(t, wantSummary.Currency, gotSummary.Currency)
	assert.Equal(t, wantSummary.ByProvider, gotSummary.ByProvider)
	assert.Equal(t, wantSummary.ByService, gotSummary.ByService)
	assert.Equal(t, wantSummary.ByAdapter, gotSummary.ByAdapter)
	for _, cost := range costs {
		response = overviewRequest(
			t,
			s,
			http.MethodGet,
			"/api/cost/actual/resource?id="+url.QueryEscape(cost.ResourceID),
			"",
		)
		var detail viewmodel.ActualCostDetail
		require.NoError(t, json.NewDecoder(response.Body).Decode(&detail))
		assert.Equal(t, cost.Breakdown, detail.Result.Breakdown)
		tuiDetail := ansi.Strip(tui.RenderDetailView(cost, 160))
		for name, value := range detail.Result.Breakdown {
			assert.Contains(t, tuiDetail, fmt.Sprintf("%s: $%.4f", name, value))
		}
	}
}

func parityRecommendations(t *testing.T, s *Server, recs []engine.Recommendation) {
	t.Helper()
	response := overviewRequest(t, s, http.MethodPost, "/api/recommendations/query", `{}`)
	var page viewmodel.RecommendationPage
	require.NoError(t, json.NewDecoder(response.Body).Decode(&page))
	assert.ElementsMatch(t, recs, page.Items)
	assert.Equal(t, engine.BuildRecommendationSummary(recs), page.Summary)
	for _, rec := range recs {
		response = overviewRequest(t, s, http.MethodGet, "/api/recommendations/item?id="+url.QueryEscape(rec.ID), "")
		var detail struct {
			Item           engine.Recommendation `json:"item"`
			SavingsDisplay string                `json:"savingsDisplay"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&detail))
		assert.Equal(t, rec, detail.Item)
		text := ansi.Strip(tui.RenderRecommendationDetail(rec, 160))
		assert.Contains(t, text, detail.SavingsDisplay)
		assert.Contains(t, text, "0.2")
	}
}

func parityEstimate(t *testing.T, s *Server, eng *engine.Engine, resource engine.ResourceDescriptor) {
	t.Helper()
	ctx := context.Background()
	overrides := map[string]string{"size": "3"}
	canonical, err := eng.EstimateCost(ctx, &engine.EstimateRequest{Resource: &resource, PropertyOverrides: overrides})
	require.NoError(t, err)
	body, err := json.Marshal(estimateQuery{URN: resource.ID, Overrides: overrides})
	require.NoError(t, err)
	response := overviewRequest(t, s, http.MethodPost, "/api/estimate/recalculate", string(body))
	require.Equal(t, http.StatusOK, response.StatusCode)
	var got struct {
		engine.EstimateResult

		Display viewmodel.EstimateDisplay `json:"display"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))
	assert.Equal(t, *canonical, got.EstimateResult)
	model := tui.NewEstimateModel(ctx, &resource, canonical)
	text := ansi.Strip(model.View().Content)
	assert.Contains(t, text, got.Display.Baseline)
	assert.Contains(t, text, got.Display.Modified)
	for _, delta := range got.Deltas {
		assert.Contains(t, text, viewmodel.FormatEstimateDelta(delta.CostChange).Text)
	}
}

// Match resource identity and cost cells within the same rendered table row.
func parityOverviewRowMatches(text string, row engine.OverviewRowResult) bool {
	for _, line := range strings.Split(text, "\n") {
		cells := strings.Fields(line)
		if !slices.Contains(cells, row.DisplayName) || !slices.Contains(cells, row.Type) {
			continue
		}
		return strings.Contains(line, row.ActualDisplay) && strings.Contains(line, row.ProjectedDisplay) &&
			strings.Contains(line, row.DeltaDisplay)
	}
	return false
}

func TestOverviewParityRejectsSwappedRowValues(t *testing.T) {
	t.Parallel()
	rows, _, _, _ := parityFixture(t, "state-no-changes.json")
	require.GreaterOrEqual(t, len(rows), 2)
	canonical := engine.ComputeOverviewResult(rows, 7)
	rows[0].ActualCost, rows[1].ActualCost = rows[1].ActualCost, rows[0].ActualCost
	rows[0].ProjectedCost, rows[1].ProjectedCost = rows[1].ProjectedCost, rows[0].ProjectedCost
	model, _ := tui.NewOverviewModel(context.Background(), rows, len(rows), nil, nil)
	updated, _ := model.Update(tui.OverviewAllResourcesLoadedMsg{})
	updated, _ = updated.Update(tea.WindowSizeMsg{Width: 300, Height: 200})
	text := ansi.Strip(updated.View().Content)
	// Whole-screen matching would still pass this wrong association.
	for _, row := range canonical.Rows[:2] {
		assert.Contains(t, text, row.ActualDisplay)
		assert.Contains(t, text, row.ProjectedDisplay)
		assert.False(t, parityOverviewRowMatches(text, row), "swapped values must fail for %s", row.URN)
	}
}
