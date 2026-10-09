package webui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestActualQueryValidation(t *testing.T) {
	t.Parallel()
	s := newRunningServer(t, Options{Session: NewSession(context.Background(), SessionOptions{})})
	for _, body := range []string{`{"groupBy":"bad"}`, `{"tag":"broken"}`, `{"tag":"=prod"}`, `{"tag":"env="}`, `{"tag":"env=prod=bad"}`, `{"sort":"bad"}`, `{"page":-1}`} {
		assert.Equal(t, http.StatusBadRequest, overviewRequest(t, s, "POST", "/api/cost/actual/query", body).StatusCode)
	}
	assert.Equal(
		t,
		http.StatusServiceUnavailable,
		overviewRequest(t, s, "POST", "/api/cost/actual/query", `{}`).StatusCode,
	)
}

func TestActualQueryParity(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	fixture := []engine.CostResult{
		{
			ResourceID:      "a",
			ResourceType:    "aws:ec2:Instance",
			TotalCost:       12,
			Monthly:         12,
			Currency:        "USD",
			StartDate:       start,
			EndDate:         start.AddDate(0, 0, 2),
			DailyCosts:      []float64{5, 7},
			Breakdown:       map[string]float64{"compute": 12},
			Sustainability:  map[string]engine.SustainabilityMetric{"carbon_footprint": {Value: 1000, Unit: "g"}},
			Recommendations: []engine.Recommendation{{ID: "rec", Type: "Resize"}},
		},
	}
	for _, group := range []string{"", "resource", "type", "provider", "daily", "monthly", "date"} {
		t.Run(group, func(t *testing.T) {
			t.Parallel()
			session := NewSession(
				context.Background(),
				SessionOptions{
					ActualCosts: func(_ context.Context, got string, tags map[string]string) (*engine.CostResultWithErrors, error) {
						assert.Equal(t, group, got)
						assert.Equal(t, map[string]string{"env": "prod"}, tags)
						return &engine.CostResultWithErrors{
							Results: engine.New(nil, nil).GroupResults(fixture, engine.GroupBy(got)),
						}, nil
					},
				},
			)
			session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
			s := newRunningServer(t, Options{Session: session})
			resp := overviewRequest(
				t,
				s,
				"POST",
				"/api/cost/actual/query",
				fmt.Sprintf(`{"groupBy":%q,"tag":"env=prod"}`, group),
			)
			require.Equal(t, 200, resp.StatusCode)
			var page struct {
				Results json.RawMessage             `json:"results"`
				Summary viewmodel.ActualCostSummary `json:"summary"`
				Carbon  greenops.CarbonInput        `json:"carbon"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
			if engine.GroupBy(group).IsTimeBasedGrouping() {
				want, err := engine.CreateCrossProviderAggregation(
					engine.New(nil, nil).GroupResults(fixture, engine.GroupBy(group)),
					engine.GroupBy(group),
				)
				require.NoError(t, err)
				var got []engine.CrossProviderAggregation
				require.NoError(t, json.Unmarshal(page.Results, &got))
				assert.ElementsMatch(t, want, got, "canonical aggregation values are unchanged")
				viewmodel.SortTimeAggregations(want, viewmodel.SortByCost)
				assert.Equal(t, want, got, "display follows the requested cost order")
			} else {
				var got []engine.CostResult
				require.NoError(t, json.Unmarshal(page.Results, &got))
				assert.Equal(t, engine.New(nil, nil).GroupResults(fixture, engine.GroupBy(group)), got)
			}
			wantCarbon, _ := engine.AggregateSustainability(
				context.Background(),
				engine.New(nil, nil).GroupResults(fixture, engine.GroupBy(group)),
			)
			assert.Equal(t, wantCarbon, page.Carbon)
			assert.Equal(t, wantCarbon, page.Summary.Carbon)
			assert.Equal(
				t,
				engine.AggregateResults(engine.New(nil, nil).GroupResults(fixture, engine.GroupBy(group))).Summary,
				page.Summary.CostSummary,
			)
		})
	}
}

func TestActualDetailGroupContext(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{
			ActualCosts: func(_ context.Context, group string, tags map[string]string) (*engine.CostResultWithErrors, error) {
				assert.Equal(t, "provider", group)
				assert.Equal(t, map[string]string{"env": "prod"}, tags)
				return &engine.CostResultWithErrors{
					Results: []engine.CostResult{
						{
							ResourceID:   "aggregated-2-resources",
							ResourceType: "aws",
							TotalCost:    12,
							Currency:     "USD",
							Breakdown:    map[string]float64{"compute": 12},
							Sustainability: map[string]engine.SustainabilityMetric{
								"carbon_footprint": {Value: 1, Unit: "kg"},
							},
							Recommendations: []engine.Recommendation{
								{ID: "resize", Type: "Resize", EstimatedSavings: 3, Currency: "USD"},
							},
						},
					},
				}, nil
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(
		t,
		s,
		"GET",
		"/api/cost/actual/resource?id=aggregated-2-resources&groupBy=provider&tag=env%3Dprod",
		"",
	)
	require.Equal(t, 200, resp.StatusCode)
	var detail map[string]json.RawMessage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	assert.Contains(t, detail, "breakdown")
	assert.Contains(t, detail, "recommendations")
	assert.Contains(t, detail, "sustainability")
	assert.Equal(t, 400, overviewRequest(t, s, "GET", "/api/cost/actual/resource?id=a&groupBy=bad", "").StatusCode)
}

func TestActualGroupedDetailDisambiguatesEqualSizedGroups(t *testing.T) {
	t.Parallel()
	fixture := []engine.CostResult{
		{ResourceID: "aggregated-2-resources", ResourceType: "aws", TotalCost: 10},
		{ResourceID: "aggregated-2-resources", ResourceType: "azure", TotalCost: 20},
	}
	session := NewSession(
		context.Background(),
		SessionOptions{
			ActualCosts: func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
				return &engine.CostResultWithErrors{Results: fixture}, nil
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(
		t,
		s,
		"GET",
		"/api/cost/actual/resource?id=aggregated-2-resources&groupBy=provider&type=azure",
		"",
	)
	require.Equal(t, 200, resp.StatusCode)
	var detail viewmodel.ActualCostDetail
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&detail))
	assert.InDelta(t, 20.0, detail.Result.TotalCost, 1e-9)
	assert.Equal(t, "azure", detail.Result.ResourceType)
}

func TestActualTrendsAndAggregationFailure(t *testing.T) {
	t.Parallel()
	fixture := []engine.CostResult{
		{ResourceID: "a", Currency: "USD", TotalCost: 1},
		{ResourceID: "b", Currency: "EUR", TotalCost: 2},
	}
	session := NewSession(
		context.Background(),
		SessionOptions{
			ActualCosts: func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
				return &engine.CostResultWithErrors{Results: fixture}, nil
			},
			Trends: func() (map[string]string, string) {
				return map[string]string{"a": "▁▂▃▄▅▆█"}, "▁▂▃▄▅▆█"
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "POST", "/api/cost/actual/query", "{}")
	require.Equal(t, 200, resp.StatusCode)
	var page viewmodel.ActualCostPage
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Equal(t, "▁▂▃▄▅▆█", page.TotalTrend)
	assert.Equal(t, "▁▂▃▄▅▆█", page.Trends["a"])
	assert.Equal(t, 502, overviewRequest(t, s, "POST", "/api/cost/actual/query", `{"groupBy":"daily"}`).StatusCode)
	assert.Equal(t, 404, overviewRequest(t, s, "GET", "/api/cost/actual/resource?id=missing", "").StatusCode)
}

func TestActualPartialErrorsRemainVisibleAndSafe(t *testing.T) {
	t.Parallel()
	session := NewSession(
		context.Background(),
		SessionOptions{
			ActualCosts: func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
				return &engine.CostResultWithErrors{
					Results: []engine.CostResult{{ResourceID: "healthy", TotalCost: 12, Currency: "USD"}},
					Errors: []engine.ErrorDetail{
						{ResourceID: "failed", PluginName: "fixture", Error: errors.New("secret-password-value")},
					},
				}, nil
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	resp := overviewRequest(t, s, "POST", "/api/cost/actual/query", "{}")
	require.Equal(t, 200, resp.StatusCode)
	var page struct {
		Results []engine.CostResult `json:"results"`
		Errors  []map[string]any    `json:"errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
	assert.Len(t, page.Results, 1)
	require.Len(t, page.Errors, 1)
	assert.Equal(t, "failed", page.Errors[0]["resourceId"])
	assert.Equal(t, "Actual cost unavailable", page.Errors[0]["message"])
}

func TestActualTimeEmptyQueryContracts(t *testing.T) {
	t.Parallel()
	for _, group := range []string{"daily", "monthly", ""} {
		for _, empty := range []string{"source", "substring", "tag"} {
			t.Run(group+"/"+empty, func(t *testing.T) {
				t.Parallel()
				session := NewSession(
					context.Background(),
					SessionOptions{
						ActualCosts: func(_ context.Context, _ string, tags map[string]string) (*engine.CostResultWithErrors, error) {
							if empty == "source" || tags["env"] == "missing" {
								return &engine.CostResultWithErrors{}, nil
							}
							return &engine.CostResultWithErrors{
								Results: []engine.CostResult{{ResourceID: "known", TotalCost: 10, Currency: "USD"}},
							}, nil
						},
					},
				)
				session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
				s := newRunningServer(t, Options{Session: session})
				query := CostQuery{GroupBy: group}
				if empty == "substring" {
					query.Filter = "missing"
				}
				if empty == "tag" {
					query.Tag = "env=missing"
				}
				body, err := json.Marshal(query)
				require.NoError(t, err)
				resp := overviewRequest(t, s, "POST", "/api/cost/actual/query", string(body))
				require.Equal(t, 200, resp.StatusCode)
				var page struct {
					Results    json.RawMessage         `json:"results"`
					Rows       []viewmodel.CostDisplay `json:"rows"`
					Page       int                     `json:"page"`
					TotalPages int                     `json:"totalPages"`
				}
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
				assert.JSONEq(t, "[]", string(page.Results))
				assert.Empty(t, page.Rows)
				assert.Equal(t, 1, page.Page)
				assert.Equal(t, 1, page.TotalPages)
			})
		}
	}
}

func TestActualTimeSortBeforePagination(t *testing.T) {
	t.Parallel()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fixture := make([]engine.CostResult, 251)
	for i := range fixture {
		day := start.AddDate(0, 0, i)
		fixture[i] = engine.CostResult{
			ResourceID:   fmt.Sprintf("r-%d", i),
			ResourceType: "aws:ec2:Instance",
			Currency:     "USD",
			TotalCost:    float64(i + 1),
			DailyCosts:   []float64{float64(i + 1)},
			StartDate:    day,
			EndDate:      day.AddDate(0, 0, 1),
		}
	}
	session := NewSession(
		context.Background(),
		SessionOptions{
			ActualCosts: func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error) {
				return &engine.CostResultWithErrors{Results: fixture}, nil
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	for _, tc := range []struct {
		sort       string
		page       int
		wantPeriod string
		want       float64
	}{{"cost", 1, start.AddDate(0, 0, 250).Format("2006-01-02"), 251}, {"cost", 2, start.Format("2006-01-02"), 1}, {"name", 1, start.Format("2006-01-02"), 1}} {
		resp := overviewRequest(
			t,
			s,
			"POST",
			"/api/cost/actual/query",
			fmt.Sprintf(`{"groupBy":"daily","sort":%q,"page":%d}`, tc.sort, tc.page),
		)
		require.Equal(t, 200, resp.StatusCode)
		var page struct {
			Results []engine.CrossProviderAggregation `json:"results"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page))
		require.NotEmpty(t, page.Results)
		assert.Equal(t, tc.wantPeriod, page.Results[0].Period)
		assert.InDelta(t, tc.want, page.Results[0].Total, 1e-9)
	}
	for _, sort := range []string{"type", "delta"} {
		assert.Equal(
			t,
			400,
			overviewRequest(
				t,
				s,
				"POST",
				"/api/cost/actual/query",
				fmt.Sprintf(`{"groupBy":"daily","sort":%q}`, sort),
			).StatusCode,
		)
	}
}
