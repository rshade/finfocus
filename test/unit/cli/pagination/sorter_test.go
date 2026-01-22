package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli/pagination"
	"github.com/rshade/finfocus/internal/engine"
)

// TestRecommendationSorter_ValidFields verifies valid sort field recognition.
func TestRecommendationSorter_ValidFields(t *testing.T) {
	sorter := pagination.NewRecommendationSorter()

	validFields := []string{
		"savings",
		"cost",
		"name",
		"resourceType",
		"provider",
		"actionType",
	}

	for _, field := range validFields {
		t.Run(field, func(t *testing.T) {
			assert.True(t, sorter.IsValidField(field), "field %s should be valid", field)
		})
	}
}

// TestRecommendationSorter_InvalidFields verifies invalid sort field detection.
func TestRecommendationSorter_InvalidFields(t *testing.T) {
	sorter := pagination.NewRecommendationSorter()

	invalidFields := []string{
		"invalid",
		"description",
		"timestamp",
		"currency",
		"",
	}

	for _, field := range invalidFields {
		t.Run(field, func(t *testing.T) {
			assert.False(t, sorter.IsValidField(field), "field %s should be invalid", field)
		})
	}
}

// TestRecommendationSorter_GetValidFields verifies valid field list.
func TestRecommendationSorter_GetValidFields(t *testing.T) {
	sorter := pagination.NewRecommendationSorter()
	fields := sorter.GetValidFields()

	expectedFields := []string{
		"savings",
		"cost",
		"name",
		"resourceType",
		"provider",
		"actionType",
	}

	assert.ElementsMatch(t, expectedFields, fields)
}

// TestRecommendationSorter_SortBySavingsDescending verifies sorting by savings descending.
func TestRecommendationSorter_SortBySavingsDescending(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "resource-1", EstimatedSavings: 100.0},
		{ResourceID: "resource-2", EstimatedSavings: 300.0},
		{ResourceID: "resource-3", EstimatedSavings: 50.0},
		{ResourceID: "resource-4", EstimatedSavings: 200.0},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "savings", "desc")

	require.Len(t, sorted, 4)
	assert.Equal(t, "resource-2", sorted[0].ResourceID) // 300.0
	assert.Equal(t, "resource-4", sorted[1].ResourceID) // 200.0
	assert.Equal(t, "resource-1", sorted[2].ResourceID) // 100.0
	assert.Equal(t, "resource-3", sorted[3].ResourceID) // 50.0
}

// TestRecommendationSorter_SortBySavingsAscending verifies sorting by savings ascending.
func TestRecommendationSorter_SortBySavingsAscending(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "resource-1", EstimatedSavings: 100.0},
		{ResourceID: "resource-2", EstimatedSavings: 300.0},
		{ResourceID: "resource-3", EstimatedSavings: 50.0},
		{ResourceID: "resource-4", EstimatedSavings: 200.0},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "savings", "asc")

	require.Len(t, sorted, 4)
	assert.Equal(t, "resource-3", sorted[0].ResourceID) // 50.0
	assert.Equal(t, "resource-1", sorted[1].ResourceID) // 100.0
	assert.Equal(t, "resource-4", sorted[2].ResourceID) // 200.0
	assert.Equal(t, "resource-2", sorted[3].ResourceID) // 300.0
}

// TestRecommendationSorter_SortByNameAscending verifies sorting by resource name.
func TestRecommendationSorter_SortByNameAscending(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "zebra-resource"},
		{ResourceID: "alpha-resource"},
		{ResourceID: "charlie-resource"},
		{ResourceID: "bravo-resource"},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "name", "asc")

	require.Len(t, sorted, 4)
	assert.Equal(t, "alpha-resource", sorted[0].ResourceID)
	assert.Equal(t, "bravo-resource", sorted[1].ResourceID)
	assert.Equal(t, "charlie-resource", sorted[2].ResourceID)
	assert.Equal(t, "zebra-resource", sorted[3].ResourceID)
}

// TestRecommendationSorter_SortByResourceTypeDescending verifies sorting by resource type.
func TestRecommendationSorter_SortByResourceTypeDescending(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "r1", Type: "RIGHTSIZE"},
		{ResourceID: "r2", Type: "TERMINATE"},
		{ResourceID: "r3", Type: "MIGRATE"},
		{ResourceID: "r4", Type: "RIGHTSIZE"},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "actionType", "desc")

	require.Len(t, sorted, 4)
	// Descending alphabetical order: TERMINATE > RIGHTSIZE > MIGRATE
	assert.Equal(t, "TERMINATE", sorted[0].Type)
	assert.True(t, sorted[1].Type == "RIGHTSIZE" || sorted[2].Type == "RIGHTSIZE")
	assert.Equal(t, "MIGRATE", sorted[3].Type)
}

// TestRecommendationSorter_SortByProviderAscending verifies sorting by provider.
func TestRecommendationSorter_SortByProviderAscending(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "aws:ec2:Instance/i-123", EstimatedSavings: 100.0},
		{ResourceID: "gcp:compute:Instance/inst-456", EstimatedSavings: 200.0},
		{ResourceID: "azure:compute:VM/vm-789", EstimatedSavings: 150.0},
		{ResourceID: "aws:rds:Database/db-012", EstimatedSavings: 300.0},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "provider", "asc")

	require.Len(t, sorted, 4)
	// Providers extracted from ResourceID: aws, azure, gcp
	assert.Contains(t, sorted[0].ResourceID, "aws")
	assert.Contains(t, sorted[1].ResourceID, "aws")
	assert.Contains(t, sorted[2].ResourceID, "azure")
	assert.Contains(t, sorted[3].ResourceID, "gcp")
}

// TestRecommendationSorter_DefaultOrderBehavior verifies behavior with invalid sort field.
func TestRecommendationSorter_DefaultOrderBehavior(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "resource-1", EstimatedSavings: 100.0},
		{ResourceID: "resource-2", EstimatedSavings: 300.0},
		{ResourceID: "resource-3", EstimatedSavings: 50.0},
	}

	sorter := pagination.NewRecommendationSorter()

	// Invalid field should return original order
	sorted := sorter.Sort(recommendations, "invalid_field", "asc")
	require.Len(t, sorted, 3)
	assert.Equal(t, "resource-1", sorted[0].ResourceID)
	assert.Equal(t, "resource-2", sorted[1].ResourceID)
	assert.Equal(t, "resource-3", sorted[2].ResourceID)
}

// TestRecommendationSorter_EmptySlice verifies handling of empty recommendation slice.
func TestRecommendationSorter_EmptySlice(t *testing.T) {
	var recommendations []engine.Recommendation

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "savings", "desc")

	assert.Len(t, sorted, 0)
}

// TestRecommendationSorter_SingleItem verifies sorting single-item slice.
func TestRecommendationSorter_SingleItem(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "resource-1", EstimatedSavings: 100.0},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "savings", "desc")

	require.Len(t, sorted, 1)
	assert.Equal(t, "resource-1", sorted[0].ResourceID)
}

// TestRecommendationSorter_StableSort verifies stable sorting with equal values.
func TestRecommendationSorter_StableSort(t *testing.T) {
	recommendations := []engine.Recommendation{
		{ResourceID: "resource-1", EstimatedSavings: 100.0},
		{ResourceID: "resource-2", EstimatedSavings: 100.0},
		{ResourceID: "resource-3", EstimatedSavings: 100.0},
	}

	sorter := pagination.NewRecommendationSorter()
	sorted := sorter.Sort(recommendations, "savings", "desc")

	require.Len(t, sorted, 3)
	// Original order should be preserved for equal values (stable sort)
	assert.Equal(t, "resource-1", sorted[0].ResourceID)
	assert.Equal(t, "resource-2", sorted[1].ResourceID)
	assert.Equal(t, "resource-3", sorted[2].ResourceID)
}

// TestParseSortExpression verifies sort expression parsing.
func TestParseSortExpression(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		wantField string
		wantOrder string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid desc",
			expr:      "savings:desc",
			wantField: "savings",
			wantOrder: "desc",
			wantErr:   false,
		},
		{
			name:      "valid asc",
			expr:      "cost:asc",
			wantField: "cost",
			wantOrder: "asc",
			wantErr:   false,
		},
		{
			name:      "field only defaults to desc",
			expr:      "name",
			wantField: "name",
			wantOrder: "desc",
			wantErr:   false,
		},
		{
			name:    "empty expression",
			expr:    "",
			wantErr: true,
			errMsg:  "empty sort expression",
		},
		{
			name:    "invalid order",
			expr:    "savings:invalid",
			wantErr: true,
			errMsg:  "invalid sort order",
		},
		{
			name:    "too many parts",
			expr:    "savings:desc:extra",
			wantErr: true,
			errMsg:  "invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, order, err := pagination.ParseSortExpression(tt.expr)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantField, field)
				assert.Equal(t, tt.wantOrder, order)
			}
		})
	}
}
