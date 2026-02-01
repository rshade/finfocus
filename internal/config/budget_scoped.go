package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Scoped budget validation errors.
var (
	// ErrGlobalBudgetRequired is returned when scoped budgets are defined but no global budget exists.
	ErrGlobalBudgetRequired = errors.New("global budget is required when scoped budgets are defined")

	// ErrCurrencyMismatch is returned when a scoped budget uses a different currency than global.
	ErrCurrencyMismatch = errors.New("scoped budget currency must match global budget currency")

	// ErrInvalidTagSelector is returned when a tag selector doesn't match the required format.
	ErrInvalidTagSelector = errors.New("invalid tag selector format")

	// ErrDuplicateTagPriority is returned when multiple tag budgets have the same priority.
	// This is a warning condition, not a hard error.
	ErrDuplicateTagPriority = errors.New("duplicate tag budget priority")
)

// Constants for budget validation.
const (
	// currencyCodeLength is the standard ISO 4217 currency code length.
	currencyCodeLength = 3

	// tagSelectorParts is the expected number of parts when splitting a tag selector by ":".
	tagSelectorParts = 2
)

// Tag selector pattern: key:value or key:* format.
// Key must be alphanumeric with optional hyphens/underscores.
// Value must be alphanumeric with optional hyphens/underscores, or * for wildcard.
var tagSelectorPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+:(\*|[a-zA-Z0-9_-]+)$`)

// ScopedBudget defines a budget limit with alert thresholds.
// It can be used for global, provider, and resource type scopes.
type ScopedBudget struct {
	// Amount is the budget limit in the specified currency.
	// Must be non-negative (zero disables the budget).
	Amount float64 `yaml:"amount" json:"amount"`

	// Currency is the ISO 4217 currency code (e.g., "USD", "EUR").
	// If empty, inherits from global budget.
	Currency string `yaml:"currency,omitempty" json:"currency,omitempty"`

	// Period defines the budget time window. Only "monthly" is supported.
	// If empty, defaults to "monthly".
	Period string `yaml:"period,omitempty" json:"period,omitempty"`

	// Alerts defines threshold percentages and their types.
	// If empty, uses default thresholds (50%, 80%, 100% actual).
	Alerts []AlertConfig `yaml:"alerts,omitempty" json:"alerts,omitempty"`

	// ExitOnThreshold overrides the global setting for this scope.
	// If nil, inherits from BudgetsConfig.ExitOnThreshold.
	ExitOnThreshold *bool `yaml:"exit_on_threshold,omitempty" json:"exit_on_threshold,omitempty"`

	// ExitCode overrides the global exit code for this scope.
	// If nil, inherits from BudgetsConfig.ExitCode.
	ExitCode *int `yaml:"exit_code,omitempty" json:"exit_code,omitempty"`
}

// IsEnabled returns true if the scoped budget is configured and enabled (Amount > 0).
// Nil-receiver behavior: ScopedBudget.IsEnabled() treats a nil receiver as not enabled
// and returns false. This allows safe nil-checking without explicit nil guards.
func (s *ScopedBudget) IsEnabled() bool {
	return s != nil && s.Amount > 0
}

// IsDisabled returns true if the scoped budget is disabled (nil or Amount == 0).
// Nil-receiver behavior: ScopedBudget.IsDisabled() treats a nil receiver as disabled/not
// configured and returns true. This semantic difference from IsEnabled is deliberate:
// "nil == not enabled" vs "nil == disabled/not configured" allows callers to check
// absence of configuration explicitly.
func (s *ScopedBudget) IsDisabled() bool {
	return s == nil || s.Amount == 0
}

// GetPeriod returns the budget period, defaulting to "monthly" if not set.
func (s *ScopedBudget) GetPeriod() string {
	if s == nil || s.Period == "" {
		return DefaultBudgetPeriod
	}
	return s.Period
}

// GetCurrency returns the configured currency or empty string if not set.
func (s *ScopedBudget) GetCurrency() string {
	if s == nil {
		return ""
	}
	return s.Currency
}

// ShouldExitOnThreshold returns true if the CLI should exit with a non-zero
// code when this scope's budget thresholds are exceeded.
// Returns nil if not explicitly set (caller should check parent/global setting).
func (s *ScopedBudget) ShouldExitOnThreshold() *bool {
	if s == nil {
		return nil
	}
	return s.ExitOnThreshold
}

// GetExitCode returns the configured exit code or nil if not set.
// Returns nil if not explicitly set (caller should check parent/global setting).
func (s *ScopedBudget) GetExitCode() *int {
	if s == nil {
		return nil
	}
	return s.ExitCode
}

// Validate checks if the scoped budget configuration is valid.
// The globalCurrency parameter is used to validate currency inheritance.
// Pass empty string for global budget validation (no inheritance check).
func (s *ScopedBudget) Validate(globalCurrency string) error {
	if s == nil {
		return nil
	}

	// Negative amounts are never allowed.
	if s.Amount < 0 {
		return ErrBudgetAmountNegative
	}

	// If budget is disabled (Amount == 0), no further validation needed.
	if s.Amount == 0 {
		return nil
	}

	// Period validation: only "monthly" is supported.
	if s.Period != "" && s.Period != DefaultBudgetPeriod {
		return fmt.Errorf("%w: got %q", ErrUnsupportedBudgetPeriod, s.Period)
	}

	// Currency validation.
	if err := s.validateCurrency(globalCurrency); err != nil {
		return err
	}

	// Validate all alert configurations.
	if err := s.validateAlerts(); err != nil {
		return err
	}

	// Validate exit code if set.
	return s.validateExitCode()
}

// validateCurrency checks if the currency code is valid and matches global if set.
func (s *ScopedBudget) validateCurrency(globalCurrency string) error {
	if s.Currency == "" {
		return nil
	}

	// Validate currency format.
	if len(s.Currency) != currencyCodeLength {
		return fmt.Errorf("invalid currency code: must be 3 letters, got %q", s.Currency)
	}
	for _, c := range s.Currency {
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("invalid currency code: must be uppercase letters, got %q", s.Currency)
		}
	}

	// Check against global currency if provided.
	if globalCurrency != "" && s.Currency != globalCurrency {
		return fmt.Errorf("%w: scope uses %s, global uses %s",
			ErrCurrencyMismatch, s.Currency, globalCurrency)
	}

	return nil
}

// validateAlerts validates all alert configurations.
func (s *ScopedBudget) validateAlerts() error {
	for i, alert := range s.Alerts {
		if err := alert.Validate(); err != nil {
			return fmt.Errorf("alert[%d]: %w", i, err)
		}
	}
	return nil
}

// validateExitCode validates the exit code if set and ExitOnThreshold is enabled.
// Exit code is only validated when ExitOnThreshold is true, since the value
// is unused when threshold-based exit is disabled.
func (s *ScopedBudget) validateExitCode() error {
	// Skip validation if exit code is not set
	if s.ExitCode == nil {
		return nil
	}
	// Skip validation if ExitOnThreshold is not enabled
	if s.ExitOnThreshold == nil || !*s.ExitOnThreshold {
		return nil
	}
	if *s.ExitCode < MinExitCode || *s.ExitCode > MaxExitCode {
		return fmt.Errorf("%w: got %d", ErrExitCodeOutOfRange, *s.ExitCode)
	}
	return nil
}

// TagBudget defines a budget scoped by a tag selector with priority ordering.
type TagBudget struct {
	// ScopedBudget embeds the budget configuration.
	// Embedded fields must be listed before regular fields per Go convention.
	ScopedBudget `yaml:",inline" json:",inline"`

	// Selector is the tag pattern in "key:value" or "key:*" format.
	// "key:value" matches exact tag values.
	// "key:*" matches any resource with the specified tag key.
	Selector string `yaml:"selector" json:"selector"`

	// Priority determines which tag budget receives cost when a resource
	// matches multiple tag selectors. Higher values take precedence.
	// If multiple budgets have the same priority, a warning is emitted
	// and the first alphabetically wins.
	Priority int `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// ParsedTagSelector represents a parsed tag selector with key and value components.
type ParsedTagSelector struct {
	// Key is the tag key to match.
	Key string
	// Value is the tag value to match, or "*" for wildcard.
	Value string
	// IsWildcard is true if the selector matches any value for the key.
	IsWildcard bool
}

// ParseTagSelector parses a tag selector string into its components.
// Valid formats: "key:value" or "key:*".
func ParseTagSelector(selector string) (*ParsedTagSelector, error) {
	if !tagSelectorPattern.MatchString(selector) {
		return nil, fmt.Errorf("%w: %q must match pattern 'key:value' or 'key:*'",
			ErrInvalidTagSelector, selector)
	}

	parts := strings.SplitN(selector, ":", tagSelectorParts)
	if len(parts) != tagSelectorParts {
		return nil, fmt.Errorf("%w: %q must contain exactly one colon",
			ErrInvalidTagSelector, selector)
	}

	return &ParsedTagSelector{
		Key:        parts[0],
		Value:      parts[1],
		IsWildcard: parts[1] == "*",
	}, nil
}

// Matches returns true if the selector matches the given tags map.
func (p *ParsedTagSelector) Matches(tags map[string]string) bool {
	if tags == nil {
		return false
	}

	value, exists := tags[p.Key]
	if !exists {
		return false
	}

	if p.IsWildcard {
		return true
	}

	return value == p.Value
}

// Validate checks if the tag budget configuration is valid.
// The globalCurrency parameter is used to validate currency inheritance.
func (t *TagBudget) Validate(globalCurrency string) error {
	if t == nil {
		return nil
	}

	// Validate selector format
	if _, err := ParseTagSelector(t.Selector); err != nil {
		return err
	}

	// Validate embedded ScopedBudget
	return t.ScopedBudget.Validate(globalCurrency)
}

// BudgetsConfig holds all budget scope configurations.
// It supports a global fallback budget and optional provider, tag, and type scopes.
type BudgetsConfig struct {
	// Global is the fallback budget that applies to all resources.
	// Required if any scoped budgets are defined.
	Global *ScopedBudget `yaml:"global,omitempty" json:"global,omitempty"`

	// Providers maps cloud provider names (aws, gcp, azure) to their budgets.
	// Provider names are case-insensitive during matching.
	Providers map[string]*ScopedBudget `yaml:"providers,omitempty" json:"providers,omitempty"`

	// Tags defines budgets scoped by resource tags with priority ordering.
	// Higher priority values take precedence when a resource matches multiple tags.
	Tags []TagBudget `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Types maps resource type patterns to their budgets.
	// Patterns use exact matching (e.g., "aws:ec2/instance").
	Types map[string]*ScopedBudget `yaml:"types,omitempty" json:"types,omitempty"`

	// ExitOnThreshold applies to all scopes unless overridden.
	ExitOnThreshold bool `yaml:"exit_on_threshold,omitempty" json:"exit_on_threshold,omitempty"`

	// ExitCode is the default exit code when thresholds are exceeded.
	// Nil means not set (defaults to 1). Zero is valid (warning-only mode).
	ExitCode *int `yaml:"exit_code,omitempty" json:"exit_code,omitempty"`
}

// HasScopedBudgets returns true if any provider, tag, or type budgets are defined.
func (b *BudgetsConfig) HasScopedBudgets() bool {
	if b == nil {
		return false
	}
	return len(b.Providers) > 0 || len(b.Tags) > 0 || len(b.Types) > 0
}

// HasGlobalBudget returns true if a global budget is configured and enabled.
func (b *BudgetsConfig) HasGlobalBudget() bool {
	return b != nil && b.Global != nil && b.Global.IsEnabled()
}

// IsEnabled returns true if any budget (global or scoped) is enabled.
func (b *BudgetsConfig) IsEnabled() bool {
	if b == nil {
		return false
	}

	if b.HasGlobalBudget() {
		return true
	}

	// Check provider budgets
	for _, p := range b.Providers {
		if p != nil && p.IsEnabled() {
			return true
		}
	}

	// Check tag budgets
	for i := range b.Tags {
		if b.Tags[i].IsEnabled() {
			return true
		}
	}

	// Check type budgets
	for _, t := range b.Types {
		if t != nil && t.IsEnabled() {
			return true
		}
	}

	return false
}

// GetGlobalCurrency returns the global budget's currency, or empty string if not set.
func (b *BudgetsConfig) GetGlobalCurrency() string {
	if b == nil || b.Global == nil {
		return ""
	}
	return b.Global.Currency
}

// GetEffectiveExitOnThreshold returns the exit_on_threshold setting for a given scope.
// It checks the scope's setting first, then falls back to the global setting.
func (b *BudgetsConfig) GetEffectiveExitOnThreshold(scopeOverride *bool) bool {
	if scopeOverride != nil {
		return *scopeOverride
	}
	if b == nil {
		return false
	}
	return b.ExitOnThreshold
}

// GetEffectiveExitCode returns the exit code for a given scope.
// It checks the scope's setting first, then falls back to the global setting.
// Returns 1 as the default if nothing is configured.
func (b *BudgetsConfig) GetEffectiveExitCode(scopeOverride *int) int {
	if scopeOverride != nil {
		return *scopeOverride
	}
	if b == nil {
		return 1
	}
	if b.ExitCode != nil {
		return *b.ExitCode
	}
	return 1
}

// Validate checks if the budgets configuration is valid.
// Returns a list of warnings (non-fatal issues) and an error for fatal issues.
func (b *BudgetsConfig) Validate() ([]string, error) {
	if b == nil {
		return nil, nil
	}

	// If scoped budgets exist, global budget is required
	if b.HasScopedBudgets() && !b.HasGlobalBudget() {
		return nil, ErrGlobalBudgetRequired
	}

	globalCurrency := b.GetGlobalCurrency()

	// Validate global budget
	if b.Global != nil {
		if validErr := b.Global.Validate(""); validErr != nil {
			return nil, fmt.Errorf("global budget: %w", validErr)
		}
	}

	if err := b.validateProviderBudgets(globalCurrency); err != nil {
		return nil, err
	}

	warnings, err := b.validateTagBudgets(globalCurrency)
	if err != nil {
		return nil, err
	}

	// Warn about tag budgets not being fully functional
	// Tag-based cost allocation requires tag data in CostResult, which is not yet implemented.
	// Users should be aware that tag budgets are configured but may not track costs correctly.
	if len(b.Tags) > 0 {
		warnings = append(warnings,
			"tag-based budgets are configured but tag allocation is not yet fully implemented; "+
				"tag budgets will show $0 spend until tag data is available in cost results")
	}

	if err = b.validateTypeBudgets(globalCurrency); err != nil {
		return nil, err
	}

	// Validate exit code
	if b.ExitCode != nil && (*b.ExitCode < MinExitCode || *b.ExitCode > MaxExitCode) {
		return nil, fmt.Errorf("%w: got %d", ErrExitCodeOutOfRange, *b.ExitCode)
	}

	return warnings, nil
}

// validateProviderBudgets validates all provider budget configurations.
func (b *BudgetsConfig) validateProviderBudgets(globalCurrency string) error {
	for name, provider := range b.Providers {
		if provider == nil {
			continue
		}
		if validErr := provider.Validate(globalCurrency); validErr != nil {
			return fmt.Errorf("provider %q budget: %w", name, validErr)
		}
	}
	return nil
}

// validateTagBudgets validates all tag budget configurations and checks for duplicate priorities.
// Returns warnings for duplicate priorities and an error for invalid tag budgets.
func (b *BudgetsConfig) validateTagBudgets(globalCurrency string) ([]string, error) {
	priorityMap := make(map[int][]string)
	for i := range b.Tags {
		tag := &b.Tags[i]
		if validErr := tag.Validate(globalCurrency); validErr != nil {
			return nil, fmt.Errorf("tag budget[%d] %q: %w", i, tag.Selector, validErr)
		}
		priorityMap[tag.Priority] = append(priorityMap[tag.Priority], tag.Selector)
	}

	var warnings []string
	for priority, selectors := range priorityMap {
		if len(selectors) > 1 {
			warnings = append(warnings,
				fmt.Sprintf("tag budgets with priority %d: %v - first alphabetically will be selected",
					priority, selectors))
		}
	}
	return warnings, nil
}

// validateTypeBudgets validates all resource type budget configurations.
func (b *BudgetsConfig) validateTypeBudgets(globalCurrency string) error {
	for typeName, typeBudget := range b.Types {
		if typeBudget == nil {
			continue
		}
		if validErr := typeBudget.Validate(globalCurrency); validErr != nil {
			return fmt.Errorf("type %q budget: %w", typeName, validErr)
		}
	}
	return nil
}
