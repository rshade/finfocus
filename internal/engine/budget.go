package engine

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// PercentageMultiplier is used to convert ratios to percentages.
const PercentageMultiplier = 100.0

// CurrencyCodeLength is the required length for valid ISO 4217 currency codes.
const CurrencyCodeLength = 3

// ErrInvalidCurrency is returned when a currency code fails validation.
var ErrInvalidCurrency = errors.New("invalid currency code")

// ErrInvalidBudget is returned when a budget fails validation.
var ErrInvalidBudget = errors.New("invalid budget")

// BudgetFilterOptions contains criteria for filtering budgets.
type BudgetFilterOptions struct {
	Providers []string          // Filter by provider names (case-insensitive, OR logic)
	Tags      map[string]string // Filter by metadata tags (case-sensitive, AND logic, supports glob patterns)
}

// BudgetResult contains the complete budget health response.
type BudgetResult struct {
	Budgets []*pbc.Budget          // Filtered budgets with health status
	Summary *ExtendedBudgetSummary // Aggregated statistics
	Errors  []error                // Any errors during processing
}

// ExtendedBudgetSummary provides detailed budget health breakdown.

type ExtendedBudgetSummary struct {
	*pbc.BudgetSummary // Embedded proto summary

	ByProvider      map[string]*pbc.BudgetSummary // Per-provider breakdown
	ByCurrency      map[string]*pbc.BudgetSummary // Per-currency breakdown
	OverallHealth   pbc.BudgetHealthStatus        // Worst-case health
	CriticalBudgets []string                      // IDs of critical/exceeded budgets
}

// BudgetHealthResult contains health assessment for a single budget.
type BudgetHealthResult struct {
	BudgetID     string                 // Budget identifier
	BudgetName   string                 // Human-readable name
	Provider     string                 // Source provider (aws-budgets, kubecost, etc.)
	Health       pbc.BudgetHealthStatus // Calculated health status
	Utilization  float64                // Current percentage used (0-100+)
	Forecasted   float64                // Forecasted percentage at period end
	Currency     string                 // ISO 4217 currency code
	Limit        float64                // Budget limit amount
	CurrentSpend float64                // Current spend amount
}

// ThresholdEvaluationResult contains evaluated threshold state.
type ThresholdEvaluationResult struct {
	Threshold   *pbc.BudgetThreshold // Original threshold
	Triggered   bool                 // Whether threshold was crossed
	TriggeredAt time.Time            // When triggered (zero if not)
	SpendType   string               // "actual" or "forecasted"
}

// FilterBudgets filters a list of budgets based on the provided criteria.
// It returns a new slice containing only the matching budgets.
// FilterBudgets returns the subset of budgets that satisfy the given BudgetFilter.
// If filter is nil, FilterBudgets returns the original budgets slice unchanged and preserves the original order.
func FilterBudgets(budgets []*pbc.Budget, filter *pbc.BudgetFilter) []*pbc.Budget {
	if filter == nil {
		return budgets
	}

	var filtered []*pbc.Budget
	for _, b := range budgets {
		if matchesBudgetFilter(b, filter) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// matchesBudgetFilter reports whether the given budget b satisfies all criteria specified in filter.
// Providers, Regions, and ResourceTypes are applied only if present in the filter: the budget must match at least one value in each of those lists (case-insensitive comparison). Tags are applied conjunctively: for each key/value pair in filter.Tags the budget's metadata must contain an entry "tag:<key>" equal to the corresponding value.
// The function reads region and resourceType from budget metadata keys "region" and "resourceType" respectively.
// Returns true if the budget meets every specified criterion in the filter, false otherwise.
func matchesBudgetFilter(b *pbc.Budget, filter *pbc.BudgetFilter) bool {
	// Provider (OR logic)
	if len(filter.GetProviders()) > 0 {
		if !matchStringSlice(b.GetSource(), filter.GetProviders()) {
			return false
		}
	}

	// Region (OR logic) - checks Metadata["region"]
	if len(filter.GetRegions()) > 0 {
		region := getMetadataValue(b, "region")
		if !matchStringSlice(region, filter.GetRegions()) {
			return false
		}
	}

	// ResourceType (OR logic) - checks Metadata["resourceType"]
	if len(filter.GetResourceTypes()) > 0 {
		resType := getMetadataValue(b, "resourceType")
		if !matchStringSlice(resType, filter.GetResourceTypes()) {
			return false
		}
	}

	// Tags (AND logic) - checks Metadata["tag:<key>"]
	if len(filter.GetTags()) > 0 {
		for key, val := range filter.GetTags() {
			// Construct the tag key as stored in metadata (e.g., "tag:env")
			metaKey := "tag:" + key
			metaVal := getMetadataValue(b, metaKey)
			if metaVal != val {
				return false
			}
		}
	}

	return true
}

// isValidCurrency reports whether currency is a three-letter ISO 4217 currency code (uppercase A–Z).
// Uses ASCII byte checks instead of regex for efficiency (no allocations).
// Returns true if the input is exactly 3 uppercase letters, false otherwise.
func isValidCurrency(code string) bool {
	if len(code) != CurrencyCodeLength {
		return false
	}
	for i := range CurrencyCodeLength {
		c := code[i]
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// ValidateCurrency checks if a currency code is valid ISO 4217 format.
//
// Valid format: exactly 3 uppercase letters (A-Z).
// Examples: "USD", "EUR", "GBP" are valid; "usd", "US", "USDD" are invalid.
//
// Returns nil if valid, error with descriptive message if invalid.
func ValidateCurrency(code string) error {
	if code == "" {
		return fmt.Errorf("%w: currency code is required", ErrInvalidCurrency)
	}
	if !isValidCurrency(code) {
		return fmt.Errorf("%w: must be 3 uppercase letters (got %q)", ErrInvalidCurrency, code)
	}
	return nil
}

// ValidateBudgetCurrency validates the currency in a budget's amount.
// Returns nil if valid or amount is nil, error otherwise.
func ValidateBudgetCurrency(budget *pbc.Budget) error {
	if budget == nil {
		return nil
	}
	amount := budget.GetAmount()
	if amount == nil {
		return nil
	}
	currency := amount.GetCurrency()
	if currency == "" {
		return nil // Empty currency is allowed (will use default)
	}
	if err := ValidateCurrency(currency); err != nil {
		return fmt.Errorf("budget %q: %w", budget.GetId(), err)
	}
	return nil
}

// ValidateBudget validates required budget fields.
func ValidateBudget(budget *pbc.Budget) error {
	if budget == nil {
		return fmt.Errorf("%w: budget is nil", ErrInvalidBudget)
	}
	if strings.TrimSpace(budget.GetId()) == "" {
		return fmt.Errorf("%w: budget id is required", ErrInvalidBudget)
	}
	if amount := budget.GetAmount(); amount != nil && amount.GetLimit() < 0 {
		return fmt.Errorf("%w: budget %q has negative limit", ErrInvalidBudget, budget.GetId())
	}
	return nil
}

func budgetID(budget *pbc.Budget) string {
	if budget == nil {
		return ""
	}
	return budget.GetId()
}

func budgetLimit(budget *pbc.Budget) float64 {
	if amount := budget.GetAmount(); amount != nil {
		return amount.GetLimit()
	}
	return 0
}

func prepareBudget(ctx context.Context, budget *pbc.Budget, periodStart time.Time, periodEnd time.Time) error {
	if err := ValidateBudget(budget); err != nil {
		return err
	}
	if err := ValidateBudgetCurrency(budget); err != nil {
		return err
	}

	ApplyDefaultThresholds(budget)

	limit := budgetLimit(budget)
	status := budget.GetStatus()
	if status != nil {
		if limit > 0 {
			status.PercentageUsed = (status.GetCurrentSpend() / limit) * PercentageMultiplier
		} else {
			status.Health = pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED
		}
	}

	UpdateBudgetForecast(ctx, budget, periodStart, periodEnd)

	if status != nil && limit > 0 {
		status.Health = CalculateBudgetHealth(budget)
		_ = EvaluateThresholds(ctx, budget, status.GetCurrentSpend(), status.GetForecastedSpend())
	}

	return nil
}

// matchStringSlice reports whether target is present in candidates using a case-insensitive comparison.
// It returns true if any element of candidates equals target ignoring letter case, and false otherwise.
func matchStringSlice(target string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(target, c) {
			return true
		}
	}
	return false
}

// getMetadataValue returns the value for the given key from the budget's metadata.
// If the metadata map is nil or the key does not exist, it returns the empty string.
func getMetadataValue(b *pbc.Budget, key string) string {
	metadata := b.GetMetadata()
	if metadata == nil {
		return ""
	}
	return metadata[key]
}

// matchesBudgetTagsWithGlob checks if a budget's metadata matches all specified tags.
// Tags are matched using AND logic: all specified tags must match for the budget to pass.
// Tag values support glob patterns (via path.Match), e.g., "prod-*" matches "prod-us", "prod-eu".
// The match is case-sensitive for both keys and values.
//
// Parameters:
//   - b: The budget to check
//   - tags: Map of tag keys to values/patterns to match against budget metadata
//
// Returns true if:
//   - tags map is nil or empty (no filtering)
//   - all specified tags match the budget's metadata (exact or glob pattern match)
//
// Returns false if:
//   - budget is nil
//   - any tag key is missing from budget metadata
//   - any tag value doesn't match (exact or glob pattern)
//
// Glob pattern errors are treated as non-matches (budget excluded).
func matchesBudgetTagsWithGlob(b *pbc.Budget, tags map[string]string) bool {
	// No tags = no filtering, all budgets pass
	if len(tags) == 0 {
		return true
	}

	// Nil budget cannot match any tags
	if b == nil {
		return false
	}

	metadata := b.GetMetadata()
	if metadata == nil {
		// No metadata means no tags can match
		return false
	}

	// AND logic: all tags must match
	for key, pattern := range tags {
		// Try both direct key and prefixed key (for legacy "tag:key" storage)
		metaVal, exists := metadata[key]
		if !exists {
			// Also check with "tag:" prefix (legacy format)
			metaVal, exists = metadata["tag:"+key]
			if !exists {
				return false
			}
		}

		// Empty pattern matches only empty value
		if pattern == "" {
			if metaVal != "" {
				return false
			}
			continue
		}

		// Try glob pattern matching
		matched, err := path.Match(pattern, metaVal)
		if err != nil {
			// Invalid pattern syntax, treat as non-match
			return false
		}
		if !matched {
			return false
		}
	}

	return true
}

// FilterBudgetsByTags filters budgets by metadata tags.
//
// Behavior:
//   - Case-sensitive matching for both keys and values
//   - AND logic: budget must match ALL specified tags
//   - Supports glob patterns in values (e.g., "prod-*" matches "prod-us")
//   - Empty tags map returns all budgets (no filtering)
//   - Returns empty slice if no budgets match
func FilterBudgetsByTags(ctx context.Context, budgets []*pbc.Budget, tags map[string]string) []*pbc.Budget {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "FilterBudgetsByTags").
		Logger()

	// If no tags specified, return all budgets
	if len(tags) == 0 {
		return budgets
	}

	logger.Debug().
		Int("tag_count", len(tags)).
		Int("input_count", len(budgets)).
		Msg("filtering budgets by tags")

	var filtered []*pbc.Budget
	for _, b := range budgets {
		if matchesBudgetTagsWithGlob(b, tags) {
			filtered = append(filtered, b)
		}
	}

	logger.Debug().
		Int("output_count", len(filtered)).
		Msg("tag filtering complete")

	return filtered
}

// FilterBudgetsByProvider filters budgets by provider name(s).
//
// Behavior:
//   - Case-insensitive matching ("aws" matches "AWS", "Aws", etc.)
//   - OR logic: budget matches if it matches ANY provider in the list
//   - Empty providers list returns all budgets (no filtering)
//   - Returns empty slice if no budgets match
func FilterBudgetsByProvider(ctx context.Context, budgets []*pbc.Budget, providers []string) []*pbc.Budget {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "FilterBudgetsByProvider").
		Logger()

	// If no providers specified, return all budgets
	if len(providers) == 0 {
		return budgets
	}

	logger.Debug().
		Strs("providers", providers).
		Int("input_count", len(budgets)).
		Msg("filtering budgets by provider")

	var filtered []*pbc.Budget
	for _, b := range budgets {
		if MatchesProvider(b, providers) {
			filtered = append(filtered, b)
		}
	}

	logger.Debug().
		Int("output_count", len(filtered)).
		Msg("filtering complete")

	return filtered
}

// MatchesProvider checks if a budget matches any of the given providers.
// Returns true if providers is empty (no filtering).
// Returns false if budget is nil.
func MatchesProvider(budget *pbc.Budget, providers []string) bool {
	if budget == nil {
		return false
	}
	// Empty providers means no filter - match all
	if len(providers) == 0 {
		return true
	}
	return matchStringSlice(budget.GetSource(), providers)
}

// GetBudgets retrieves budgets from plugins and applies health calculations.
// This is the main entry point for budget health functionality.
func (e *Engine) GetBudgets(ctx context.Context, filter *BudgetFilterOptions) (*BudgetResult, error) {
	logger := logging.FromContext(ctx).With().
		Str("component", "engine").
		Str("operation", "GetBudgets").
		Logger()

	result := &BudgetResult{}

	var allBudgets []*pbc.Budget

	// 1. Query plugins
	for _, client := range e.clients {
		resp, err := client.API.GetBudgets(ctx, &pbc.GetBudgetsRequest{})
		if err != nil {
			logger.Warn().Str("plugin", client.Name).Err(err).Msg("failed to get budgets from plugin")
			result.Errors = append(result.Errors, fmt.Errorf("plugin %s: %w", client.Name, err))
			continue
		}
		if resp != nil && len(resp.GetBudgets()) > 0 {
			allBudgets = append(allBudgets, resp.GetBudgets()...)
		}
	}

	if len(allBudgets) == 0 && len(result.Errors) > 0 {
		return result, fmt.Errorf("failed to retrieve budgets from any plugin: %v", result.Errors)
	}

	// 2. Filter budgets
	// Apply provider filter first (OR logic), then tag filter (AND logic)
	var providers []string
	var tags map[string]string
	if filter != nil {
		providers = filter.Providers
		tags = filter.Tags
	}
	filteredBudgets := FilterBudgetsByProvider(ctx, allBudgets, providers)
	filteredBudgets = FilterBudgetsByTags(ctx, filteredBudgets, tags)

	// 3. Process each budget
	now := time.Now()
	// Determine current period start/end.
	// For MVP, we assume monthly budgets and calculate for current month.
	// In future, we might read period from budget object or request.
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var validBudgets []*pbc.Budget
	for _, b := range filteredBudgets {
		if err := prepareBudget(ctx, b, periodStart, periodEnd); err != nil {
			logger.Warn().
				Str("budget_id", budgetID(b)).
				Err(err).
				Msg("invalid budget, skipping")
			result.Errors = append(result.Errors, err)
			continue
		}

		validBudgets = append(validBudgets, b)
	}

	result.Budgets = validBudgets

	// 4. Summarize
	result.Summary = CalculateExtendedSummary(ctx, validBudgets)

	logger.Info().
		Int("total_budgets", len(allBudgets)).
		Int("filtered_budgets", len(filteredBudgets)).
		Str("overall_health", result.Summary.OverallHealth.String()).
		Msg("budget retrieval complete")

	return result, nil
}
