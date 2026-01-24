package engine

import (
	"context"
	"errors"
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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

func TestEngine_GetBudgets(t *testing.T) {
	ctx := context.Background()

	// Setup mock data
	budget1 := &pbc.Budget{
		Id:     "b1",
		Source: "aws-budgets",
		Amount: &pbc.BudgetAmount{Limit: 1000},
		Status: &pbc.BudgetStatus{CurrentSpend: 500},
	}
	budget2 := &pbc.Budget{
		Id:     "b2",
		Source: "gcp-billing",
		Amount: &pbc.BudgetAmount{Limit: 2000},
		Status: &pbc.BudgetStatus{CurrentSpend: 2500}, // Exceeded
	}

	tests := []struct {
		name          string
		clients       []*pluginhost.Client
		filter        *BudgetFilterOptions
		expectedCount int
		expectError   bool
		checkFunc     func(t *testing.T, result *BudgetResult)
	}{
		{
			name:          "no plugins",
			clients:       []*pluginhost.Client{},
			expectedCount: 0,
		},
		{
			name: "single plugin success",
			clients: []*pluginhost.Client{
				{
					Name: "aws-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget1}},
				},
			},
			expectedCount: 1,
			checkFunc: func(t *testing.T, result *BudgetResult) {
				assert.Equal(t, "b1", result.Budgets[0].GetId())
				// Verify health calculation happened (implied by non-nil status health)
				assert.Equal(
					t,
					pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
					result.Budgets[0].GetStatus().GetHealth(),
				)
			},
		},
		{
			name: "multiple plugins aggregation",
			clients: []*pluginhost.Client{
				{
					Name: "aws-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget1}},
				},
				{
					Name: "gcp-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget2}},
				},
			},
			expectedCount: 2,
			checkFunc: func(t *testing.T, result *BudgetResult) {
				findBudget := func(id string) *pbc.Budget {
					for _, budget := range result.Budgets {
						if budget.GetId() == id {
							return budget
						}
					}
					return nil
				}

				budgetOne := findBudget("b1")
				require.NotNil(t, budgetOne)
				assert.Equal(t, "aws-budgets", budgetOne.GetSource())

				budgetTwo := findBudget("b2")
				require.NotNil(t, budgetTwo)
				assert.Equal(t, "gcp-billing", budgetTwo.GetSource())
				// Verify summary aggregation
				assert.Equal(t, int32(2), result.Summary.GetTotalBudgets())
				assert.Equal(t, int32(1), result.Summary.GetBudgetsOk())
				assert.Equal(t, int32(1), result.Summary.GetBudgetsExceeded())
			},
		},
		{
			name: "provider filter applied",
			clients: []*pluginhost.Client{
				{
					Name: "aws-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget1}},
				},
				{
					Name: "gcp-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget2}},
				},
			},
			filter:        &BudgetFilterOptions{Providers: []string{"aws-budgets"}},
			expectedCount: 1,
			checkFunc: func(t *testing.T, result *BudgetResult) {
				assert.Equal(t, "b1", result.Budgets[0].GetId())
			},
		},
		{
			name: "plugin error handling (partial failure)",
			clients: []*pluginhost.Client{
				{
					Name: "aws-plugin",
					API:  &mockCostSourceClient{budgets: []*pbc.Budget{budget1}},
				},
				{
					Name: "error-plugin",
					API:  &mockCostSourceClient{err: errors.New("rpc error")},
				},
			},
			expectedCount: 1, // Should return partial results
			checkFunc: func(t *testing.T, result *BudgetResult) {
				assert.Equal(t, "b1", result.Budgets[0].GetId())
				assert.Len(t, result.Errors, 1)
				assert.Contains(t, result.Errors[0].Error(), "rpc error")
			},
		},
		{
			name: "all plugins fail",
			clients: []*pluginhost.Client{
				{
					Name: "error-plugin-1",
					API:  &mockCostSourceClient{err: errors.New("error 1")},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng := New(tc.clients, nil)
			result, err := eng.GetBudgets(ctx, tc.filter)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Len(t, result.Budgets, tc.expectedCount)
				if tc.checkFunc != nil {
					tc.checkFunc(t, result)
				}
			}
		})
	}
}
