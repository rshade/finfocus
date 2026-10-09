package tui

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
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
	b.WriteString(viewmodel.FormatPricingDetails(mode))
	if len(m.pricingModes) > 1 {
		b.WriteString("←/→: Billing mode\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatPricingRate(mode engine.PricingMode) string { return viewmodel.FormatPricingRate(mode) }
