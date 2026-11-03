package viewmodel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestEstimateDisplayCarriesSpotLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		modified     *engine.CostResult
		wantBaseline string
		wantModified string
	}{
		{
			name: "dynamic with risk",
			modified: &engine.CostResult{
				Monthly: 20, PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
			},
			wantModified: "SPOT · risk 0.12 · spot savings unavailable",
		},
		{
			name:         "dynamic zero risk",
			modified:     &engine.CostResult{Monthly: 20, PricingCategory: engine.PricingCategoryDynamic},
			wantModified: "SPOT · risk unknown · spot savings unavailable",
		},
		{
			name: "dynamic with savings",
			modified: func() *engine.CostResult {
				onDemand := 100.0
				result := &engine.CostResult{
					Monthly: 38, PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.12,
					OnDemandEquivalentMonthly: &onDemand,
				}
				result.SpotSavingsPercent = engine.SpotSavingsPercent(result.PricingCategory, result.Monthly, &onDemand)
				return result
			}(),
			wantModified: "SPOT · risk 0.12 · -62% vs on-demand",
		},
		{
			name:     "standard",
			modified: &engine.CostResult{Monthly: 20, PricingCategory: engine.PricingCategoryStandard},
		},
		{
			name:     "unspecified",
			modified: &engine.CostResult{Monthly: 20},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			display := BuildEstimateDisplay(context.Background(), nil, &engine.EstimateResult{
				Baseline: &engine.CostResult{Monthly: 60},
				Modified: tt.modified,
			}, nil)
			assert.Equal(t, tt.wantBaseline, display.BaselineSpot)
			assert.Equal(t, tt.wantModified, display.ModifiedSpot)
		})
	}
}
