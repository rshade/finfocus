package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffResultStackAfterCostsIncludesUnchangedResources(t *testing.T) {
	t.Parallel()

	var nilResult *DiffResult
	assert.Nil(t, nilResult.StackAfterCosts())

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
