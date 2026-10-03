package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractProviderFromResourceType_EdgeCases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "unknown", ExtractProviderFromResourceType(""))
	assert.Equal(t, "unknown", ExtractProviderFromResourceType(":missing"))
	assert.Equal(t, "aws", ExtractProviderFromResourceType("aws:ec2:Instance"))
	assert.Equal(t, "aws", ExtractProviderFromResourceType("aws_instance"))
	assert.Equal(t, "custom", ExtractProviderFromResourceType("custom"))
}

func TestProjectedPropertiesForRow_PrefersProjected(t *testing.T) {
	t.Parallel()

	got := projectedPropertiesForRow(OverviewRow{
		ProjectedProperties: map[string]any{"sku": "t3.small"},
		Properties:          map[string]any{"sku": "t3.micro"},
	})
	assert.Equal(t, "t3.small", got["sku"])

	fallback := projectedPropertiesForRow(OverviewRow{
		Properties: map[string]any{"sku": "t3.micro"},
	})
	assert.Equal(t, "t3.micro", fallback["sku"])
}

func TestDriftElapsedDays_EmptyWindow(t *testing.T) {
	t.Parallel()

	ref := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	assert.Zero(t, driftElapsedDays(ref, ref, nil))
	after := ref.Add(time.Hour)
	assert.Zero(t, driftElapsedDays(after, ref, nil))
}

func TestEnrichActualCost_ReportsErrorResultsAndDefaultsCurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	row := &OverviewRow{URN: "urn:actual", Type: "aws:ec2:Instance", Status: StatusActive}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN, Provider: "aws"}
	window := DateRange{Start: time.Now().Add(-24 * time.Hour), End: time.Now()}

	skipped := enrichActualCost(ctx, row, &mockEnricher{
		actualResult: &CostResultWithErrors{Results: []CostResult{{Notes: "ERROR: no price"}}},
	}, resource, window)
	require.NotNil(t, skipped)
	assert.Nil(t, row.ActualCost)

	validation := enrichActualCost(ctx, row, &mockEnricher{
		actualResult: &CostResultWithErrors{Results: []CostResult{{Notes: "VALIDATION: bad sku"}}},
	}, resource, window)
	require.NotNil(t, validation)
	assert.Nil(t, row.ActualCost)

	structured := enrichActualCost(ctx, row, &mockEnricher{
		actualResult: &CostResultWithErrors{Results: []CostResult{{Error: &StructuredError{}}}},
	}, resource, window)
	require.NotNil(t, structured)
	assert.Nil(t, row.ActualCost)

	noData := enrichActualCost(ctx, row, &mockEnricher{
		actualResult: &CostResultWithErrors{Results: []CostResult{{
			Error: &StructuredError{Code: ErrCodeNoCostData, Message: "No pricing information available"},
		}}},
	}, resource, window)
	require.Nil(t, noData)
	assert.Nil(t, row.ActualCost)

	ok := enrichActualCost(ctx, row, &mockEnricher{
		actualResult: &CostResultWithErrors{Results: []CostResult{{TotalCost: 4.5}}},
	}, resource, window)
	require.Nil(t, ok)
	require.NotNil(t, row.ActualCost)
	assert.Equal(t, defaultCurrency, row.ActualCost.Currency)
	assert.InDelta(t, 4.5, row.ActualCost.MTDCost, 0.001)

	fetchErr := enrichActualCost(ctx, row, &mockEnricher{actualErr: errors.New("connection reset")}, resource, window)
	require.NotNil(t, fetchErr)
	assert.Equal(t, ErrorTypeNetwork, fetchErr.ErrorType)
	assert.Contains(t, fetchErr.Message, "connection reset")
}

func TestEnrichProjectedCost_ReportsErrorResultsDefaultsCurrencyAndLogsZero(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	row := &OverviewRow{URN: "urn:projected", Type: "aws:ec2:Instance", Status: StatusActive}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN, Provider: "aws"}

	skipped := enrichProjectedCost(ctx, row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{Notes: "ERROR: no price"}}},
	}, resource)
	require.NotNil(t, skipped)
	assert.Nil(t, row.ProjectedCost)

	validation := enrichProjectedCost(ctx, row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{Notes: "VALIDATION: bad"}}},
	}, resource)
	require.NotNil(t, validation)

	ok := enrichProjectedCost(ctx, row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 0}}},
	}, resource)
	require.Nil(t, ok)
	require.NotNil(t, row.ProjectedCost)
	assert.Equal(t, defaultCurrency, row.ProjectedCost.Currency)
	assert.Zero(t, row.ProjectedCost.MonthlyCost)

	fetchErr := enrichProjectedCost(ctx, row, &mockEnricher{
		projectedErr: errors.New("permission denied"),
	}, resource)
	require.NotNil(t, fetchErr)
	assert.Equal(t, ErrorTypeAuth, fetchErr.ErrorType)
}

func TestEnrichBaselineProjectedCost_Branches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	row := &OverviewRow{URN: "urn:base", Type: "aws:ec2:Instance"}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN}

	enrichBaselineProjectedCost(ctx, row, &mockEnricher{projectedErr: errors.New("down")}, resource)
	assert.Nil(t, row.BaselineProjectedCost)

	enrichBaselineProjectedCost(ctx, row, &mockEnricher{}, resource)
	assert.Nil(t, row.BaselineProjectedCost)

	enrichBaselineProjectedCost(ctx, row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{Notes: "VALIDATION: skip"}}},
	}, resource)
	assert.Nil(t, row.BaselineProjectedCost)

	enrichBaselineProjectedCost(ctx, row, &mockEnricher{
		projectedResult: &CostResultWithErrors{Results: []CostResult{{Monthly: 9}}},
	}, resource)
	require.NotNil(t, row.BaselineProjectedCost)
	assert.Equal(t, defaultCurrency, row.BaselineProjectedCost.Currency)
	assert.InDelta(t, 9.0, row.BaselineProjectedCost.MonthlyCost, 0.001)
}

func TestEnrichRecommendations_ErrorAndSuccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	row := &OverviewRow{URN: "urn:recs", Type: "aws:ec2:Instance"}
	resource := ResourceDescriptor{Type: row.Type, ID: row.URN}

	enrichRecommendations(ctx, row, &mockEnricher{recommendErr: errors.New("unavailable")}, resource)
	assert.Empty(t, row.Recommendations)

	enrichRecommendations(ctx, row, &mockEnricher{
		recommendResult: &RecommendationsResult{Recommendations: []Recommendation{{ID: "rec-1"}}},
	}, resource)
	require.Len(t, row.Recommendations, 1)
	assert.Equal(t, "rec-1", row.Recommendations[0].ID)
}
