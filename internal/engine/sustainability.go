package engine

import (
	"context"

	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/logging"
)

// AggregateSustainability aggregates carbon footprint metrics from the given
// cost results. It scans each result's Sustainability map for the canonical
// carbon metric key or a deprecated fallback, normalizes found values to
// kilograms, and sums them.
// ctx enables trace ID propagation for warning logs.
// Invalid or unnormalizable units are logged and skipped.
// It returns a CarbonInput containing the total carbon in kilograms and `true`
// if any carbon data was found; otherwise it returns a zero-value CarbonInput
// and `false`.
func AggregateSustainability(ctx context.Context, results []CostResult) (greenops.CarbonInput, bool) {
	totalCarbon := 0.0
	found := false

	for _, r := range results {
		if r.Sustainability == nil {
			continue
		}

		// Check for canonical key first.
		metric, ok := r.Sustainability[greenops.CarbonMetricKey]
		if !ok {
			// Fallback to deprecated key.
			metric, ok = r.Sustainability[greenops.DeprecatedCarbonKey]
		}

		if ok {
			// Normalize to kg before summing.
			kg, err := greenops.NormalizeToKg(metric.Value, metric.Unit)
			if err != nil {
				logging.FromContext(ctx).Warn().
					Ctx(ctx).
					Str("component", "engine").
					Str("operation", "aggregate_sustainability").
					Str("resource_type", r.ResourceType).
					Str("resource_id", r.ResourceID).
					Str("unit", metric.Unit).
					Err(err).
					Msg("skipped resource due to NormalizeToKg error")
				continue
			}
			totalCarbon += kg
			found = true
		}
	}

	// Always use kg as we normalize all values to kilograms.
	unit := ""
	if found {
		unit = "kg"
	}

	return greenops.CarbonInput{Value: totalCarbon, Unit: unit}, found
}
