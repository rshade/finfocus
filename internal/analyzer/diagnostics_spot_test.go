package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestCostToDiagnostic_SpotLabel(t *testing.T) {
	t.Parallel()

	onDemand := 57.63
	base := engine.CostResult{
		ResourceType: "aws:ec2/instance:Instance",
		ResourceID:   "batch-worker",
		Adapter:      "aws-public",
		Currency:     "USD",
		Monthly:      21.90,
	}
	const urn = "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::batch-worker"
	plainMessage := CostToDiagnostic(base, urn, "0.1.0").GetMessage()

	t.Run("dynamic with savings carries the shared label", func(t *testing.T) {
		t.Parallel()
		spot := base
		spot.PricingCategory = engine.PricingCategoryDynamic
		spot.SpotInterruptionRisk = 0.12
		spot.OnDemandEquivalentMonthly = &onDemand
		spot.SpotSavingsPercent = engine.SpotSavingsPercent(spot.PricingCategory, spot.Monthly, &onDemand)

		message := CostToDiagnostic(spot, urn, "0.1.0").GetMessage()
		require.NotEmpty(t, spot.SpotDisplay())
		assert.Contains(t, message, "SPOT · risk 0.12 · -62% vs on-demand")
		assert.Contains(t, message, "Estimated Monthly Cost: $21.90 USD (source: aws-public)")
		assert.Contains(t, message, "<!-- finfocus:cost:")
	})

	t.Run("dynamic without risk or savings reads unknown and unavailable", func(t *testing.T) {
		t.Parallel()
		spot := base
		spot.PricingCategory = engine.PricingCategoryDynamic

		message := CostToDiagnostic(spot, urn, "0.1.0").GetMessage()
		assert.Contains(t, message, spot.SpotDisplay())
		assert.Contains(t, message, "risk unknown")
	})

	t.Run("non-dynamic categories stay byte-identical", func(t *testing.T) {
		t.Parallel()
		for _, category := range []engine.PricingCategory{
			"", engine.PricingCategoryStandard, engine.PricingCategoryCommitted, engine.PricingCategoryOther,
		} {
			cost := base
			cost.PricingCategory = category
			assert.Equal(t, plainMessage, CostToDiagnostic(cost, urn, "0.1.0").GetMessage(), "category %q", category)
		}
	})
}
