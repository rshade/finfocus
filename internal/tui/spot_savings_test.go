package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestPricingSnapshotKeepsSavings(t *testing.T) {
	t.Parallel()

	onDemand := 100.0
	result := &engine.CostResult{
		Monthly:                   38,
		Currency:                  "USD",
		PricingCategory:           engine.PricingCategoryDynamic,
		SpotInterruptionRisk:      0.12,
		OnDemandEquivalentMonthly: &onDemand,
		SpotSavingsPercent:        engine.SpotSavingsPercent(engine.PricingCategoryDynamic, 38, &onDemand),
	}
	snapshot := snapshotPricing(result)
	assert.Equal(t, "SPOT · risk 0.12 · -62% vs on-demand", snapshot.display())

	rebuilt := snapshot.costResult(38, "USD")
	require.NotNil(t, rebuilt.OnDemandEquivalentMonthly)
	assert.Equal(t, snapshot.display(), rebuilt.SpotDisplay())

	unavailable := snapshotPricing(&engine.CostResult{PricingCategory: engine.PricingCategoryDynamic})
	assert.Equal(t, "SPOT · risk unknown · spot savings unavailable", unavailable.display())
}
