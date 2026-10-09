package viewmodel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

// FormatPricingDetails renders plugin tiers, assumptions and usage hints using
// the shared estimate TUI presentation without client-side rate formatting.
func FormatPricingDetails(mode engine.PricingMode) string {
	var b strings.Builder
	writePricingTiers(&b, mode.Tiers)
	writePricingAssumptions(&b, mode.Assumptions)
	writePricingHints(&b, mode.MetricHints)
	return b.String()
}

func writePricingTiers(b *strings.Builder, tiers []engine.PricingTierOption) {
	if len(tiers) == 0 {
		return
	}
	b.WriteString("Pricing tiers:\n")
	for _, tier := range tiers {
		upper := strconv.FormatFloat(tier.MaxQuantity, 'f', -1, 64)
		if tier.MaxQuantity == 0 {
			upper = "+"
		}
		fmt.Fprintf(
			b,
			"  %s-%s: %s %s\n",
			strconv.FormatFloat(tier.MinQuantity, 'f', -1, 64),
			upper,
			strconv.FormatFloat(tier.RatePerUnit, 'f', -1, 64),
			tier.Description,
		)
	}
}

func writePricingAssumptions(b *strings.Builder, assumptions []string) {
	if len(assumptions) == 0 {
		return
	}
	b.WriteString("Assumptions:\n")
	for _, item := range assumptions {
		b.WriteString("  - ")
		b.WriteString(item)
		b.WriteByte('\n')
	}
}

func writePricingHints(b *strings.Builder, hints []engine.PricingMetricHint) {
	if len(hints) == 0 {
		return
	}
	b.WriteString("Usage:\n")
	for _, hint := range hints {
		if hint.Unit == "" {
			fmt.Fprintf(b, "  %s\n", hint.Metric)
			continue
		}
		fmt.Fprintf(b, "  %s (%s)\n", hint.Metric, hint.Unit)
	}
}
