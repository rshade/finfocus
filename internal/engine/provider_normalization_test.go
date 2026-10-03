package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterResourcesProviderUsesTheCloud(t *testing.T) {
	t.Parallel()

	resources := []ResourceDescriptor{
		{ID: "classic", Type: "aws:ec2/instance:Instance"},
		{ID: "native", Type: "aws-native:ec2:Instance"},
		{ID: "gce", Type: "google-native:compute/v1:Instance"},
		{ID: "terraform", Type: "azurerm_linux_virtual_machine"},
	}

	tests := []struct {
		filter string
		want   []string
	}{
		{"provider=aws", []string{"classic", "native"}},
		{"provider=aws-native", []string{"classic", "native"}},
		{"provider=gcp", []string{"gce"}},
		{"provider=azure", []string{"terraform"}},
		{"provider=AWS", []string{"classic", "native"}},
		{"provider=a", nil},
	}

	for _, tt := range tests {
		t.Run(tt.filter, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, resource := range FilterResources(resources, tt.filter) {
				got = append(got, resource.ID)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProviderAggregationGroupsByCloud(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	results := []CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "a",
			Monthly:      100,
			Currency:     "USD",
			StartDate:    start,
			EndDate:      start.AddDate(0, 0, 1),
		},
		{
			ResourceType: "aws-native:ec2:Instance",
			ResourceID:   "b",
			Monthly:      50,
			Currency:     "USD",
			StartDate:    start,
			EndDate:      start.AddDate(0, 0, 1),
		},
		{
			ResourceType: "azure-native:compute:VirtualMachine",
			ResourceID:   "c",
			Monthly:      30,
			Currency:     "USD",
			StartDate:    start,
			EndDate:      start.AddDate(0, 0, 1),
		},
	}

	t.Run("summary ByProvider", func(t *testing.T) {
		t.Parallel()
		summary := AggregateResults(results).Summary
		assert.Equal(t, map[string]float64{"aws": 150, "azure": 30}, summary.ByProvider)
	})

	t.Run("GroupByProvider", func(t *testing.T) {
		t.Parallel()
		grouped := (&Engine{}).GroupResults(results, GroupByProvider)
		require.Len(t, grouped, 2)

		byType := map[string]CostResult{}
		for _, group := range grouped {
			byType[group.ResourceType] = group
		}
		require.Contains(t, byType, "aws")
		assert.Equal(t, "aggregated-2-resources", byType["aws"].ResourceID)
		assert.InDelta(t, 150.0, byType["aws"].Monthly, 1e-9)
	})
}
