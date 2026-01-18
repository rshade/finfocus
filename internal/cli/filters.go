package cli

import (
	"context"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
)

// ApplyFilters validates and applies a slice of filter strings to a resource set.
// It logs validation failures and filter application results for debugging.
//
// The function performs two passes:
//  1. Validation: All filters are validated upfront. If any filter is invalid,
//     an error is returned immediately without applying any filters.
//  2. Application: Valid filters are applied sequentially, reducing the resource set.
//
// An empty filter slice returns the original resources unchanged.
// A warning is logged if the filtered result is empty.
//
// Filter syntax follows engine.ValidateFilter rules: "key=value" format
// ApplyFilters validates and applies a sequence of filter expressions to a slice of ResourceDescriptor.
// It first validates all non-empty filters; if any validation fails, it returns nil and that validation error.
// If the filters slice is empty, the original resources are returned unchanged.
// Valid, non-empty filters are applied sequentially to the resource set and the resulting slice is returned.
// ctx is used for cancellation and logging. resources is the input set to filter. filters is the ordered list
// of filter expressions; empty strings in filters are ignored.
// It returns the filtered resources and a nil error on success, or nil and a validation error if any filter is invalid.
func ApplyFilters(
	ctx context.Context,
	resources []engine.ResourceDescriptor,
	filters []string,
) ([]engine.ResourceDescriptor, error) {
	log := logging.FromContext(ctx)

	if len(filters) == 0 {
		return resources, nil
	}

	// Validate all filters upfront
	for _, f := range filters {
		if f == "" {
			continue
		}
		if err := engine.ValidateFilter(f); err != nil {
			log.Warn().Ctx(ctx).
				Str("component", "cli").
				Str("operation", "apply_filters").
				Str("filter", f).
				Err(err).
				Msg("invalid filter expression")
			return nil, err
		}
	}

	// Apply filters sequentially
	result := resources
	for _, f := range filters {
		if f == "" {
			continue
		}
		before := len(result)
		result = engine.FilterResources(result, f)
		log.Debug().Ctx(ctx).
			Str("component", "cli").
			Str("operation", "apply_filters").
			Str("filter", f).
			Int("before", before).
			Int("after", len(result)).
			Msg("applied filter")
	}

	if len(result) == 0 && len(resources) > 0 {
		log.Warn().Ctx(ctx).
			Str("component", "cli").
			Str("operation", "apply_filters").
			Int("original_count", len(resources)).
			Msg("no resources match filter criteria")
	}

	return result, nil
}