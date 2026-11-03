package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffRowNote_SpotLabel(t *testing.T) {
	t.Parallel()

	spot := &CostResult{PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.12}
	onDemand := &CostResult{PricingCategory: PricingCategoryStandard}
	tests := []struct {
		name  string
		entry DiffEntry
		want  string
	}{
		{"spot to on-demand", DiffEntry{Operation: DiffOperationUpdate, Before: spot, After: onDemand}, ""},
		{
			"on-demand to spot",
			DiffEntry{Operation: DiffOperationUpdate, Before: onDemand, After: spot},
			"SPOT · risk 0.12 · spot savings unavailable",
		},
		{
			"spot to spot",
			DiffEntry{Operation: DiffOperationUpdate, Before: spot, After: spot},
			"SPOT · risk 0.12 · spot savings unavailable",
		},
		{
			"delete of spot",
			DiffEntry{Operation: DiffOperationDelete, Before: spot},
			"SPOT · risk 0.12 · spot savings unavailable",
		},
		{
			"create of spot",
			DiffEntry{Operation: DiffOperationCreate, After: spot},
			"SPOT · risk 0.12 · spot savings unavailable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, diffRowNote(&tt.entry))
		})
	}
}
