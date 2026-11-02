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
			wantModified: "SPOT · risk 0.12",
		},
		{
			name:         "dynamic zero risk",
			modified:     &engine.CostResult{Monthly: 20, PricingCategory: engine.PricingCategoryDynamic},
			wantModified: "SPOT · risk unknown",
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
