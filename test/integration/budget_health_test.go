package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// mockCostSourceClient implements proto.CostSourceClient for testing.
type mockCostSourceClient struct {
	budgets []*pbc.Budget
	err     error
	name    string
}

func (m *mockCostSourceClient) GetBudgets(
	ctx context.Context,
	in *pbc.GetBudgetsRequest,
	opts ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &pbc.GetBudgetsResponse{Budgets: m.budgets}, nil
}

func (m *mockCostSourceClient) Name(
	ctx context.Context,
	in *proto.Empty,
	opts ...grpc.CallOption,
) (*proto.NameResponse, error) {
	return &proto.NameResponse{Name: m.name}, nil
}

// Stubs for other interface methods.
func (m *mockCostSourceClient) GetProjectedCost(
	ctx context.Context,
	in *proto.GetProjectedCostRequest,
	opts ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	return &proto.GetProjectedCostResponse{}, nil
}

func (m *mockCostSourceClient) GetActualCost(
	ctx context.Context,
	in *proto.GetActualCostRequest,
	opts ...grpc.CallOption,
) (*proto.GetActualCostResponse, error) {
	return &proto.GetActualCostResponse{}, nil
}

func (m *mockCostSourceClient) GetRecommendations(
	ctx context.Context,
	in *proto.GetRecommendationsRequest,
	opts ...grpc.CallOption,
) (*proto.GetRecommendationsResponse, error) {
	return &proto.GetRecommendationsResponse{}, nil
}

func (m *mockCostSourceClient) GetPluginInfo(
	ctx context.Context,
	in *proto.Empty,
	opts ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	return &pbc.GetPluginInfoResponse{}, nil
}

func (m *mockCostSourceClient) DryRun(
	ctx context.Context,
	in *pbc.DryRunRequest,
	opts ...grpc.CallOption,
) (*pbc.DryRunResponse, error) {
	return &pbc.DryRunResponse{}, nil
}

func (m *mockCostSourceClient) DismissRecommendation(
	ctx context.Context,
	in *proto.DismissRecommendationRequest,
	opts ...grpc.CallOption,
) (*proto.DismissRecommendationResponse, error) {
	return &proto.DismissRecommendationResponse{Success: true}, nil
}

func TestBudgetHealth_EndToEnd(t *testing.T) {
	ctx := context.Background()

	// 1. Setup Mock Data
	// Budget 1: OK (50% used)
	b1 := &pbc.Budget{
		Id:     "b-ok",
		Source: "aws-budgets",
		Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
		Status: &pbc.BudgetStatus{CurrentSpend: 500},
	}

	// Budget 2: Exceeded (120% used)
	b2 := &pbc.Budget{
		Id:     "b-exceeded",
		Source: "gcp-billing",
		Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
		Status: &pbc.BudgetStatus{CurrentSpend: 1200},
	}

	// Budget 3: Critical (95% used)
	b3 := &pbc.Budget{
		Id:     "b-critical",
		Source: "aws-budgets",
		Amount: &pbc.BudgetAmount{Limit: 1000, Currency: "USD"},
		Status: &pbc.BudgetStatus{CurrentSpend: 950},
	}

	// 2. Setup Engine with Mocks
	clients := []*pluginhost.Client{
		{
			Name: "aws-plugin",
			API:  &mockCostSourceClient{budgets: []*pbc.Budget{b1, b3}},
		},
		{
			Name: "gcp-plugin",
			API:  &mockCostSourceClient{budgets: []*pbc.Budget{b2}},
		},
	}

	eng := engine.New(clients, nil)

	// 3. Execute GetBudgets (No Filter)
	result, err := eng.GetBudgets(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 4. Verify Aggregated Results
	assert.Len(t, result.Budgets, 3)

	// Verify Summary
	summary := result.Summary
	require.NotNil(t, summary)
	assert.Equal(t, int32(3), summary.GetTotalBudgets())
	assert.Equal(t, int32(1), summary.GetBudgetsOk())       // b1
	assert.Equal(t, int32(1), summary.GetBudgetsCritical()) // b3
	assert.Equal(t, int32(1), summary.GetBudgetsExceeded()) // b2
	assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, summary.OverallHealth)

	// Verify Provider Breakdown
	assert.Contains(t, summary.ByProvider, "aws-budgets")
	assert.Equal(t, int32(2), summary.ByProvider["aws-budgets"].GetTotalBudgets())
	assert.Contains(t, summary.ByProvider, "gcp-billing")
	assert.Equal(t, int32(1), summary.ByProvider["gcp-billing"].GetTotalBudgets())

	// 5. Verify Filter Logic
	filter := &engine.BudgetFilterOptions{
		Providers: []string{"aws-budgets"},
	}
	filteredResult, err := eng.GetBudgets(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, filteredResult.Budgets, 2)
	ids := []string{
		filteredResult.Budgets[0].GetId(),
		filteredResult.Budgets[1].GetId(),
	}
	assert.ElementsMatch(t, []string{"b-ok", "b-critical"}, ids)

	// Verify Summary for Filtered
	assert.Equal(t, int32(2), filteredResult.Summary.GetTotalBudgets())
	assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL, filteredResult.Summary.OverallHealth)

	// 6. Verify Health Calculation Logic Integration
	// Check b3 (Critical) in the result
	var criticalBudget *pbc.Budget
	for _, b := range result.Budgets {
		if b.GetId() == "b-critical" {
			criticalBudget = b
			break
		}
	}
	require.NotNil(t, criticalBudget)
	assert.Equal(t, pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL, criticalBudget.GetStatus().GetHealth())

	// Verify Thresholds were applied (Defaults: 50, 80, 100)
	// 950/1000 = 95% -> Should trigger 50% and 80% thresholds
	require.NotEmpty(t, criticalBudget.GetThresholds())
	percentages := make([]float64, 0, len(criticalBudget.GetThresholds()))
	triggeredCount := 0
	for _, t := range criticalBudget.GetThresholds() {
		percentages = append(percentages, t.GetPercentage())
		if t.GetTriggered() {
			triggeredCount++
		}
	}
	assert.Equal(t, []float64{50, 80, 100}, percentages)
	assert.Equal(t, 2, triggeredCount, "Expected 50% and 80% thresholds to be triggered")

	// 7. Verify Forecasting Integration
	// Since we can't easily control time.Now() inside Engine.GetBudgets without injection,
	// we just verify that fields are populated (not zero).
	// Current implementation calculates forecast for current month.
	// Unless current spend is 0, forecast should be > 0.
	assert.Greater(t, criticalBudget.GetStatus().GetForecastedSpend(), 0.0)
	assert.Greater(t, criticalBudget.GetStatus().GetPercentageForecasted(), 0.0)
}
