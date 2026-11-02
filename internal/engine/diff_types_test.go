package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffResultStackAfterCostsIncludesUnchangedResources(t *testing.T) {
	t.Parallel()

	var nilResult *DiffResult
	require.Nil(t, nilResult.StackAfterCosts())

	result := &DiffResult{
		Entries: []DiffEntry{
			{ResourceID: "changed", After: &CostResult{ResourceID: "changed", Monthly: 10}},
			{ResourceID: "deleted"},
		},
		StackUnchanged: []DiffEntry{
			{ResourceID: "same", After: &CostResult{ResourceID: "same", Monthly: 5}},
			{ResourceID: "unpriced"},
		},
	}
	costs := result.StackAfterCosts()
	ids := make([]string, 0, len(costs))
	for _, cost := range costs {
		ids = append(ids, cost.ResourceID)
	}
	assert.Equal(
		t,
		[]string{"changed", "same"},
		ids,
		"plan entries first, then unchanged; entries with no after cost are skipped",
	)
	assert.Len(t, result.AfterCosts(), 1, "AfterCosts still covers only the plan entries")
}

func TestDiffResultStackAfterCostsSkipsDeletedResources(t *testing.T) {
	t.Parallel()

	result := &DiffResult{
		Entries: []DiffEntry{
			{
				Operation:  DiffOperationDelete,
				ResourceID: "gone",
				After:      &CostResult{ResourceID: "gone", Currency: "EUR"},
			},
			{
				Operation:  DiffOperationCreate,
				ResourceID: "free",
				After:      &CostResult{ResourceID: "free", Currency: "USD"},
			},
		},
		StackUnchanged: []DiffEntry{
			{ResourceID: "same", After: &CostResult{ResourceID: "same", Currency: "USD", Monthly: 5}},
		},
	}
	ids := make([]string, 0, 3)
	for _, cost := range result.StackAfterCosts() {
		ids = append(ids, cost.ResourceID)
	}
	assert.Equal(t, []string{"free", "same"}, ids,
		"a deleted resource is not live, and its currency must not reach the forecast; a live $0 resource stays")
}
