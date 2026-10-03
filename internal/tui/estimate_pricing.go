package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

func (m *EstimateModel) renderPricingSection() string {
	if m.pricingSpecLoading {
		return "Pricing spec: loading..."
	}
	if len(m.pricingModes) == 0 {
		return ""
	}
	selected := m.pricingMode
	if selected < 0 || selected >= len(m.pricingModes) {
		selected = 0
	}
	var b strings.Builder
	b.WriteString("Billing mode:\n")
	for i, mode := range m.pricingModes {
		mark := "  "
		if i == selected {
			mark = "* "
		}
		fmt.Fprintf(&b, "%s%s  %s  %s\n", mark, mode.BillingMode, formatPricingRate(mode), mode.Plugin)
	}
	mode := m.pricingModes[selected]
	writePricingTiers(&b, mode.Tiers)
	writePricingAssumptions(&b, mode.Assumptions)
	writePricingHints(&b, mode.MetricHints)
	if len(m.pricingModes) > 1 {
		b.WriteString("←/→: Billing mode\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatPricingRate(mode engine.PricingMode) string {
	currency := mode.Currency
	if currency == "" {
		currency = defaultEstimateCurrency
	}
	unit := mode.Unit
	if unit == "" {
		unit = mode.BillingMode
	}
	return currency + " " + strconv.FormatFloat(mode.RatePerUnit, 'f', -1, 64) + "/" + unit
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
