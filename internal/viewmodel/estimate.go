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
	// Class is pricing or other; identity rows are listed in EstimateDisplay.Details.
	Class          PropertyClass   `json:"class"`
	Provenance     InputProvenance `json:"provenance,omitempty"`
	Editable       bool            `json:"editable"`
	DefaultVisible bool            `json:"defaultVisible"`
	Overridden     bool            `json:"overridden"`
	// Usage is true for a reviewed plugin-input the descriptor does not carry.
	Usage bool `json:"usage,omitempty"`
}

// EstimateDisplay contains the presentation shared by estimate transports.
type EstimateDisplay struct {
	Baseline string `json:"baseline"`
	Modified string `json:"modified"`
	Change   string `json:"change"`
	Arrow    string `json:"arrow"`
	// BaselineSpot and ModifiedSpot are engine.SpotDisplay for each side, empty when not spot.
	BaselineSpot string             `json:"baselineSpot,omitempty"`
	ModifiedSpot string             `json:"modifiedSpot,omitempty"`
	Properties   []EstimateProperty `json:"properties"`
	// Details is read-only identity metadata, sorted by key.
	Details []EstimateDetail `json:"details"`
	// Curated is true when the type has reviewed pricing inputs present.
	Curated bool `json:"curated"`
	// Note is the fallback text when Curated is false.
	Note string `json:"note,omitempty"`
}

// NewEstimatePropertyDisplay projects a plan and its deltas into the display
// fields every transport shares. Deltas are formatted only when withDeltas is true.
func NewEstimatePropertyDisplay(plan EstimatePropertyPlan, deltas []engine.CostDelta, withDeltas bool) EstimateDisplay {
	if withDeltas {
		ApplyEstimateDeltas(plan.Rows, deltas)
	}
	display := EstimateDisplay{
		Properties: make([]EstimateProperty, 0, len(plan.Rows)),
		Details:    plan.Details,
		Curated:    plan.Curated,
		Note:       plan.Note,
	}
	if display.Details == nil {
		display.Details = []EstimateDetail{}
	}
	for _, row := range plan.Rows {
		property := EstimateProperty{
			Key: row.Key, OriginalValue: row.OriginalValue, CurrentValue: row.CurrentValue,
			Class: row.Class, Provenance: row.Provenance, Editable: row.Editable,
			DefaultVisible: row.DefaultVisible, Overridden: row.Overridden, Usage: row.Usage,
		}
		if withDeltas {
			property.Delta = FormatEstimateDelta(row.CostDelta).Text
		}
		display.Properties = append(display.Properties, property)
	}
	return display
}

// BuildEstimateDisplay formats estimates using shared TUI conventions. Sensitive
// source properties are removed before conversion to strings. Hints are the
// usage metrics discovered for the resource type, or nil when discovery failed.
func BuildEstimateDisplay(
	ctx context.Context,
	resource *engine.ResourceDescriptor,
	result *engine.EstimateResult,
	overrides map[string]string,
	input EstimateInputContext,
) EstimateDisplay {
	display := EstimateDisplay{Properties: []EstimateProperty{}, Details: []EstimateDetail{}}
	if result == nil {
		return display
	}
	currency := defaultCurrency
	if result.Baseline != nil {
		if result.Baseline.Currency != "" {
			currency = result.Baseline.Currency
		}
		display.Baseline = FormatEstimateMonthly(result.Baseline.Monthly, currency)
		display.BaselineSpot = result.Baseline.SpotDisplay()
	}
	if result.Modified != nil {
		display.Modified = FormatEstimateMonthly(result.Modified.Monthly, currency)
		display.ModifiedSpot = result.Modified.SpotDisplay()
	}
	delta := FormatEstimateDelta(result.TotalChange)
	display.Change = delta.Text
	display.Arrow = delta.Arrow
	if resource == nil {
		return display
	}
	plan := BuildEstimatePropertyPlan(ctx, resource, overrides, input)
	shared := NewEstimatePropertyDisplay(plan, result.Deltas, true)
	display.Properties = shared.Properties
	display.Details = shared.Details
	display.Curated = shared.Curated
	display.Note = shared.Note

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
	// The fields below are set by BuildEstimatePropertyPlan.
	Class          PropertyClass
	Provenance     InputProvenance
	Editable       bool
	DefaultVisible bool
	Overridden     bool
	// Usage is true for a reviewed plugin-input the descriptor does not carry.
	Usage bool
}

// SetCurrentValue records an edit and refreshes Overridden and DefaultVisible
// with the rule BuildEstimatePropertyPlan applies. curated is the plan's
// Curated flag. Every editor (terminal and web) uses this one rule.
func (r *EstimatePropertyRow) SetCurrentValue(value string, curated bool) {
	r.CurrentValue = value
	r.refreshFlags(curated)
}

// refreshFlags derives Overridden and DefaultVisible: a pricing input, an
// edited row, or any row of an uncurated type is shown by default.
func (r *EstimatePropertyRow) refreshFlags(curated bool) {
	r.Overridden = r.CurrentValue != r.OriginalValue
	r.DefaultVisible = r.Class == PropertyClassPricing || r.Overridden || !curated
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
