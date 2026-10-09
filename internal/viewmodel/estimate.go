package viewmodel

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/rshade/finfocus/internal/engine"
)

const estimateCentsMultiplier = 100

// EstimateDelta contains the shared TUI delta text and semantic direction.
type EstimateDelta struct {
	Text  string
	Arrow string
}

// FormatEstimateDelta preserves the estimate TUI's cent rounding and arrow convention.
func FormatEstimateDelta(delta float64) EstimateDelta {
	rounded := math.Round(delta*estimateCentsMultiplier) / estimateCentsMultiplier
	arrow, sign := "→", ""
	if rounded > 0 {
		arrow, sign = "↑", "+"
	} else if rounded < 0 {
		arrow = "↓"
	}
	return EstimateDelta{Text: fmt.Sprintf("%s$%.2f %s", sign, math.Abs(rounded), arrow), Arrow: arrow}
}

// FormatEstimateMonthly preserves the TUI monthly price and currency label.
func FormatEstimateMonthly(cost float64, currency string) string {
	return fmt.Sprintf("%s%.2f/mo (%s)", CurrencySymbol(currency), cost, currency)
}

// EstimateProperty carries safe editable values and a server-formatted cost delta.
type EstimateProperty struct {
	Key           string `json:"key"`
	OriginalValue string `json:"originalValue"`
	CurrentValue  string `json:"currentValue"`
	Delta         string `json:"delta"`
}

// EstimateDisplay contains the presentation shared by estimate transports.
type EstimateDisplay struct {
	Baseline   string             `json:"baseline"`
	Modified   string             `json:"modified"`
	Change     string             `json:"change"`
	Arrow      string             `json:"arrow"`
	Properties []EstimateProperty `json:"properties"`
}

// BuildEstimateDisplay formats estimates using shared TUI conventions. Sensitive
// source properties are removed before conversion to strings.
func BuildEstimateDisplay(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	result *engine.EstimateResult,
	overrides map[string]string,
) EstimateDisplay {
	display := EstimateDisplay{Properties: []EstimateProperty{}}
	if result == nil {
		return display
	}
	currency := defaultCurrency
	if result.Baseline != nil {
		if result.Baseline.Currency != "" {
			currency = result.Baseline.Currency
		}
		display.Baseline = FormatEstimateMonthly(result.Baseline.Monthly, currency)
	}
	if result.Modified != nil {
		display.Modified = FormatEstimateMonthly(result.Modified.Monthly, currency)
	}
	delta := FormatEstimateDelta(result.TotalChange)
	display.Change = delta.Text
	display.Arrow = delta.Arrow
	if resource == nil {
		return display
	}
	props := engine.BuildAttributes(ctx, resource.Properties).AsMap()
	rows := BuildEstimatePropertyRows(props, overrides)
	ApplyEstimateDeltas(rows, result.Deltas)
	for _, row := range rows {
		display.Properties = append(display.Properties, EstimateProperty{
			Key: row.Key, OriginalValue: row.OriginalValue, CurrentValue: row.CurrentValue,
			Delta: FormatEstimateDelta(row.CostDelta).Text,
		})
	}

	return display
}

// FormatPricingRate preserves the estimate TUI pricing-spec rate representation.
func FormatPricingRate(mode engine.PricingMode) string {
	currency := mode.Currency
	if currency == "" {
		currency = defaultCurrency
	}
	unit := mode.Unit
	if unit == "" {
		unit = mode.BillingMode
	}
	return currency + " " + strconv.FormatFloat(mode.RatePerUnit, 'f', -1, 64) + "/" + unit
}

// EstimatePropertyRow is a transport-neutral editable estimate property.
type EstimatePropertyRow struct {
	Key           string
	OriginalValue string
	CurrentValue  string
	CostDelta     float64
}

// BuildEstimatePropertyRows applies the shared property order and value projection.
// Callers redact properties before presentation when required by their transport.
func BuildEstimatePropertyRows(properties map[string]any, overrides map[string]string) []EstimatePropertyRow {
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	rows := make([]EstimatePropertyRow, 0, len(keys))
	for _, key := range keys {
		original := fmt.Sprintf("%v", properties[key])
		current := original
		if value, ok := overrides[key]; ok {
			current = value
		}
		rows = append(rows, EstimatePropertyRow{Key: key, OriginalValue: original, CurrentValue: current})
	}
	return rows
}

// ApplyEstimateDeltas resets stale amounts and projects matching per-property deltas.
// Unmatched combined deltas remain represented only in the total change.
func ApplyEstimateDeltas(rows []EstimatePropertyRow, deltas []engine.CostDelta) {
	for i := range rows {
		rows[i].CostDelta = 0
		for _, delta := range deltas {
			if delta.Property == rows[i].Key {
				rows[i].CostDelta = delta.CostChange
				break
			}
		}
	}
}
