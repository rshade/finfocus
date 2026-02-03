package cli

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
)

// MaxBudgetFilters is the maximum number of filter arguments allowed.
// This prevents potential DoS from excessive filter arguments.
const MaxBudgetFilters = 100

// MaxBudgetTags is the maximum number of tag filters allowed.
// This prevents memory exhaustion from unbounded tag map growth.
const MaxBudgetTags = 50

// ErrInvalidBudgetFilter is returned when a budget filter has invalid syntax.
var ErrInvalidBudgetFilter = errors.New("invalid budget filter syntax")

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
// (e.g., "type=aws:ec2/instance", "tag:env=prod").
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

// ParseBudgetFilters parses a slice of filter strings into BudgetFilterOptions.
// Filter syntax:
//   - "provider=<name>": Filter by provider (e.g., "provider=kubecost")
//   - "tag:<key>=<value>": Filter by metadata tag (e.g., "tag:namespace=production")
//
// Provider filters use OR logic (any provider matches).
// Tag filters use AND logic (all tags must match).
// Tag values support glob patterns (e.g., "tag:namespace=prod-*").
//
// Returns an error if any filter has invalid syntax per ValidateBudgetFilter,
// or if input limits are exceeded (MaxBudgetFilters, MaxBudgetTags).
func ParseBudgetFilters(ctx context.Context, filters []string) (*engine.BudgetFilterOptions, error) {
	log := logging.FromContext(ctx)

	// Check input limits to prevent DoS
	if len(filters) > MaxBudgetFilters {
		return nil, fmt.Errorf("%w: too many filters (max %d, got %d)",
			ErrInvalidBudgetFilter, MaxBudgetFilters, len(filters))
	}

	opts := &engine.BudgetFilterOptions{
		Tags: make(map[string]string),
	}

	for _, f := range filters {
		if f == "" {
			continue
		}

		// Validate filter syntax
		if err := ValidateBudgetFilter(f); err != nil {
			log.Debug().Ctx(ctx).
				Str("component", "cli").
				Str("operation", "parse_budget_filters").
				Str("filter", f).
				Err(err).
				Msg("invalid budget filter")
			return nil, err
		}

		// Parse and add filter to options
		if err := parseSingleBudgetFilter(ctx, f, opts); err != nil {
			return nil, err
		}
	}

	log.Debug().Ctx(ctx).
		Str("component", "cli").
		Str("operation", "parse_budget_filters").
		Strs("providers", opts.Providers).
		Int("tag_count", len(opts.Tags)).
		Msg("parsed budget filters")

	return opts, nil
}

// parseSingleBudgetFilter parses a single validated filter into options.
func parseSingleBudgetFilter(ctx context.Context, f string, opts *engine.BudgetFilterOptions) error {
	log := logging.FromContext(ctx)

	// Parse provider filter: provider=<name>
	if strings.HasPrefix(f, "provider=") {
		provider := strings.TrimPrefix(f, "provider=")
		opts.Providers = append(opts.Providers, provider)
		return nil
	}

	// Parse tag filter: tag:<key>=<value>
	if strings.HasPrefix(f, "tag:") {
		return parseBudgetTagFilter(ctx, f, opts)
	}

	// Unknown filter type - ValidateBudgetFilter should have caught this
	log.Warn().Ctx(ctx).
		Str("component", "cli").
		Str("operation", "parse_budget_filters").
		Str("filter", f).
		Msg("unknown filter type (ignored)")

	return nil
}

// parseBudgetTagFilter parses a tag filter and adds it to options.
func parseBudgetTagFilter(ctx context.Context, f string, opts *engine.BudgetFilterOptions) error {
	log := logging.FromContext(ctx)

	// Check tag limit before adding
	if len(opts.Tags) >= MaxBudgetTags {
		return fmt.Errorf("%w: too many tag filters (max %d)",
			ErrInvalidBudgetFilter, MaxBudgetTags)
	}

	tagPart := strings.TrimPrefix(f, "tag:")
	idx := strings.Index(tagPart, "=")
	if idx <= 0 {
		return nil // ValidateBudgetFilter already ensured valid syntax
	}

	key := tagPart[:idx]
	value := tagPart[idx+1:]

	// Warn if overwriting existing key
	if existingValue, exists := opts.Tags[key]; exists && existingValue != value {
		log.Warn().Ctx(ctx).
			Str("component", "cli").
			Str("operation", "parse_budget_filters").
			Str("key", key).
			Str("existing_value", existingValue).
			Str("new_value", value).
			Msg("duplicate tag key in filters, later value overwrites")
	}
	opts.Tags[key] = value

	return nil
}

// ValidateBudgetFilter validates a single budget filter string.
// Valid formats:
//   - "provider=<value>": Provider filter (value required)
//   - "tag:<key>=<value>": Tag filter (key required, value can be empty, supports glob patterns)
//
// Returns an error for invalid syntax:
//   - Missing "=" in tag filter
//   - Empty key after "tag:"
//   - Invalid glob pattern in tag value
//   - Unknown filter prefix
func ValidateBudgetFilter(filter string) error {
	if filter == "" {
		return fmt.Errorf("%w: empty filter", ErrInvalidBudgetFilter)
	}

	// Provider filter: provider=<value>
	if strings.HasPrefix(filter, "provider=") {
		return validateProviderFilter(filter)
	}

	// Tag filter: tag:<key>=<value>
	if strings.HasPrefix(filter, "tag:") {
		return validateTagFilter(filter)
	}

	// Unknown filter type
	return fmt.Errorf("%w: unknown filter type in %q (expected 'provider=' or 'tag:')", ErrInvalidBudgetFilter, filter)
}

// validateProviderFilter validates a provider= filter.
func validateProviderFilter(filter string) error {
	value := strings.TrimPrefix(filter, "provider=")
	if value == "" {
		return fmt.Errorf("%w: missing provider value in %q", ErrInvalidBudgetFilter, filter)
	}
	return nil
}

// validateTagFilter validates a tag: filter including glob pattern syntax.
func validateTagFilter(filter string) error {
	tagPart := strings.TrimPrefix(filter, "tag:")

	// Must contain "="
	idx := strings.Index(tagPart, "=")
	if idx < 0 {
		return fmt.Errorf("%w: missing '=' in %q", ErrInvalidBudgetFilter, filter)
	}

	// Key must not be empty
	key := tagPart[:idx]
	if key == "" {
		return fmt.Errorf("%w: empty key in %q", ErrInvalidBudgetFilter, filter)
	}

	// Validate glob pattern syntax if value is non-empty
	value := tagPart[idx+1:]
	if value != "" {
		if err := validateGlobPattern(value); err != nil {
			return fmt.Errorf("%w: invalid glob pattern %q in %q: %w",
				ErrInvalidBudgetFilter, value, filter, err)
		}
	}

	return nil
}

// validateGlobPattern checks if a pattern is valid for path.Match.
func validateGlobPattern(pattern string) error {
	_, err := path.Match(pattern, "")
	return err
}
